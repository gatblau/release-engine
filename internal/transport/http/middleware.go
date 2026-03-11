package http

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Context key for auth claims
type contextKey string

const (
	authClaimsKey contextKey = "auth_claims"
	tenantIDKey   contextKey = "tenant_id"
)

// AuthClaims represents the JWT claims extracted from a valid token.
type AuthClaims struct {
	Subject   string `json:"sub"`
	TenantID  string `json:"tenant_id"`
	Role      string `json:"role"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

// Error definitions for middleware.
var (
	ErrAuthMissing            = errors.New("missing bearer token")
	ErrAuthInvalid            = errors.New("invalid token")
	ErrRateLimited            = errors.New("rate limit exceeded")
	ErrRateLimiterUnavailable = errors.New("rate limiter unavailable")
)

// MiddlewareError represents a typed error from middleware.
type MiddlewareError struct {
	Err    error
	Code   string
	Detail map[string]any
}

func (e *MiddlewareError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *MiddlewareError) Unwrap() error {
	return e.Err
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key.
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// TokenBucket represents a token bucket for rate limiting.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refill     float64
	lastRefill time.Time
}

// RateLimiterStore manages token buckets per tenant.
type RateLimiterStore struct {
	mu       sync.RWMutex
	buckets  map[string]*TokenBucket
	capacity float64
	refill   float64
	logger   *zap.Logger
}

// NewRateLimiterStore creates a new rate limiter store.
func NewRateLimiterStore(capacity float64, refill float64, logger *zap.Logger) *RateLimiterStore {
	return &RateLimiterStore{
		buckets:  make(map[string]*TokenBucket),
		capacity: capacity,
		refill:   refill,
		logger:   logger,
	}
}

// GetBucket gets or creates a token bucket for a tenant.
func (r *RateLimiterStore) GetBucket(tenantID string) *TokenBucket {
	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, exists := r.buckets[tenantID]
	if !exists {
		bucket = &TokenBucket{
			tokens:     r.capacity,
			capacity:   r.capacity,
			refill:     r.refill,
			lastRefill: time.Now(),
		}
		r.buckets[tenantID] = bucket
	}
	return bucket
}

// Consume attempts to consume one token from the bucket.
// Returns true if successful, false if rate limited.
func (b *TokenBucket) Consume() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = b.tokens + (elapsed * b.refill)
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RemainingTokens returns the number of remaining tokens.
func (b *TokenBucket) RemainingTokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	tokens := b.tokens + (elapsed * b.refill)
	if tokens > b.capacity {
		tokens = b.capacity
	}
	return tokens
}

// RetryAfter returns the seconds until the next token is available.
func (b *TokenBucket) RetryAfter() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	tokensNeeded := 1 - b.tokens
	if tokensNeeded <= 0 {
		return 0
	}
	return int(tokensNeeded/b.refill) + 1
}

// JWKSCache caches JWKS and refreshes periodically.
type JWKSCache struct {
	mu           sync.RWMutex
	keys         map[string]*rsa.PublicKey
	issuerURL    string
	refreshAfter time.Time
	logger       *zap.Logger
	httpClient   *http.Client
}

// NewJWKSCache creates a new JWKS cache.
func NewJWKSCache(issuerURL string, logger *zap.Logger) *JWKSCache {
	return &JWKSCache{
		keys:       make(map[string]*rsa.PublicKey),
		issuerURL:  issuerURL,
		logger:     logger,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetKey retrieves a public key by key ID, fetching from the server if needed.
func (c *JWKSCache) GetKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	// Check local cache first
	c.mu.RLock()
	key, exists := c.keys[keyID]
	needsRefresh := time.Now().After(c.refreshAfter)
	c.mu.RUnlock()

	if exists && !needsRefresh {
		return key, nil
	}

	// Refresh JWKS
	if err := c.refresh(ctx); err != nil {
		// If refresh fails but we have a cached key, use it
		if exists {
			c.logger.Warn("JWKS refresh failed, using cached key",
				zap.String("key_id", keyID),
				zap.Error(err))
			return key, nil
		}
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	// Try to get the key again
	c.mu.RLock()
	key, exists = c.keys[keyID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	return key, nil
}

// refresh fetches the JWKS from the issuer and updates the cache.
func (c *JWKSCache) refresh(ctx context.Context) error {
	jwksURL := strings.TrimSuffix(c.issuerURL, "/") + "/.well-known/jwks.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create JWKS request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close JWKS response body", zap.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err = json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Parse and cache keys
	newKeys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		// Decode modulus and exponent
		var nBytes []byte
		nBytes, err = base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			c.logger.Warn("failed to decode modulus", zap.Error(err))
			continue
		}

		var eBytes []byte
		eBytes, err = base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			c.logger.Warn("failed to decode exponent", zap.Error(err))
			continue
		}

		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		publicKey := &rsa.PublicKey{N: n, E: e}
		newKeys[jwk.Kid] = publicKey
	}

	// Update cache
	c.mu.Lock()
	c.keys = newKeys
	c.refreshAfter = time.Now().Add(5 * time.Minute)
	c.mu.Unlock()

	return nil
}

// authMiddleware validates JWT bearer tokens.
type authMiddleware struct {
	issuerURL string
	audience  string
	jwksCache *JWKSCache
	logger    *zap.Logger
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(issuerURL string, audience string, logger *zap.Logger) echo.MiddlewareFunc {
	mw := &authMiddleware{
		issuerURL: issuerURL,
		audience:  audience,
		jwksCache: NewJWKSCache(issuerURL, logger),
		logger:    logger,
	}

	return mw.Process
}

// Process validates the JWT token and injects claims into context.
func (m *authMiddleware) Process(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		authHeader := c.Request().Header.Get("Authorization")

		// Step 1: Extract Bearer token
		if authHeader == "" {
			m.logger.Warn("authentication failed: missing authorization header")
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "missing bearer token",
				Code:    "AUTH_MISSING",
				Details: nil,
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			m.logger.Warn("authentication failed: invalid authorization header format")
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "missing bearer token",
				Code:    "AUTH_MISSING",
				Details: nil,
			})
		}

		tokenString := parts[1]

		// Step 2: Parse JWT to extract key ID
		parser := jwt.Parser{}
		token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			m.logger.Warn("authentication failed: invalid token format", zap.Error(err))
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid token",
				Code:    "AUTH_INVALID",
				Details: nil,
			})
		}

		// Get key ID from header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			m.logger.Warn("authentication failed: missing key ID in token")
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid token",
				Code:    "AUTH_INVALID",
				Details: nil,
			})
		}

		// Step 3: Fetch public key
		publicKey, err := m.jwksCache.GetKey(ctx, kid)
		if err != nil {
			m.logger.Warn("authentication failed: failed to get public key",
				zap.String("kid", kid),
				zap.Error(err))
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid token",
				Code:    "AUTH_INVALID",
				Details: nil,
			})
		}

		// Step 4: Validate token
		keyFunc := func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return publicKey, nil
		}

		claims := &jwt.MapClaims{}
		token, err = jwt.ParseWithClaims(tokenString, claims, keyFunc,
			jwt.WithIssuer(m.issuerURL),
			jwt.WithAudience(m.audience),
		)

		if err != nil {
			m.logger.Warn("authentication failed: token validation failed", zap.Error(err))
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid token",
				Code:    "AUTH_INVALID",
				Details: nil,
			})
		}

		if !token.Valid {
			m.logger.Warn("authentication failed: token invalid")
			return echo.NewHTTPError(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid token",
				Code:    "AUTH_INVALID",
				Details: nil,
			})
		}

		// Step 5: Extract and inject claims into context
		authClaims := AuthClaims{
			Subject:   (*claims)["sub"].(string),
			TenantID:  (*claims)["tenant_id"].(string),
			Role:      claimAsString(*claims, "role"),
			Issuer:    (*claims)["iss"].(string),
			Audience:  (*claims)["aud"].(string),
			ExpiresAt: int64((*claims)["exp"].(float64)),
			IssuedAt:  int64((*claims)["iat"].(float64)),
		}

		// Set in echo context
		c.Set(string(authClaimsKey), authClaims)
		if authClaims.TenantID != "" {
			c.Set(string(tenantIDKey), authClaims.TenantID)
		}

		m.logger.Debug("authentication successful",
			zap.String("subject", authClaims.Subject),
			zap.String("tenant_id", authClaims.TenantID))

		return next(c)
	}
}

// rateLimiterMiddleware applies per-tenant token bucket rate limiting.
type rateLimiterMiddleware struct {
	store  *RateLimiterStore
	logger *zap.Logger
}

// NewRateLimiterMiddleware creates a new RateLimiter middleware.
func NewRateLimiterMiddleware(capacity float64, refill float64, logger *zap.Logger) echo.MiddlewareFunc {
	mw := &rateLimiterMiddleware{
		store:  NewRateLimiterStore(capacity, refill, logger),
		logger: logger,
	}

	return mw.Process
}

// Process applies rate limiting based on tenant ID.
func (m *rateLimiterMiddleware) Process(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get tenant ID from context (set by AuthMiddleware) or use default
		tenantID := c.Get(string(tenantIDKey))
		if tenantID == nil {
			tenantID = "default"
		}

		tenantStr, ok := tenantID.(string)
		if !ok {
			tenantStr = "default"
		}

		// Get token bucket for this tenant
		bucket := m.store.GetBucket(tenantStr)

		// Try to consume a token
		if !bucket.Consume() {
			retryAfter := bucket.RetryAfter()
			m.logger.Warn("rate limit exceeded",
				zap.String("tenant_id", tenantStr),
				zap.Float64("remaining", bucket.RemainingTokens()),
				zap.Int("retry_after", retryAfter))

			// Return 429 with Retry-After header
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			return echo.NewHTTPError(http.StatusTooManyRequests, ErrorResponse{
				Error:   "rate limit exceeded",
				Code:    "ERR_RATE_LIMITED",
				Details: nil,
			})
		}

		m.logger.Debug("rate limit check passed",
			zap.String("tenant_id", tenantStr),
			zap.Float64("remaining", bucket.RemainingTokens()))

		return next(c)
	}
}

// GetAuthClaims retrieves auth claims from the echo context.
func GetAuthClaims(c echo.Context) (AuthClaims, bool) {
	claims, ok := c.Get(string(authClaimsKey)).(AuthClaims)
	return claims, ok
}

// GetTenantID retrieves the tenant ID from the echo context.
func GetTenantID(c echo.Context) string {
	tenantID, ok := c.Get(string(tenantIDKey)).(string)
	if !ok {
		return ""
	}
	return tenantID
}

func claimAsString(claims jwt.MapClaims, key string) string {
	value, ok := claims[key]
	if !ok {
		return ""
	}
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}
	return stringValue
}
