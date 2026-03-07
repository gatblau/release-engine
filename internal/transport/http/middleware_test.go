package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap/zaptest"
)

// TestAuthMiddleware_MissingToken tests that missing token returns 401
func TestAuthMiddleware_MissingToken(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := NewAuthMiddleware("https://issuer.example.com", "test-audience", logger)

	e := echo.New()
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}

	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", httpErr.Code)
	}

	// Check error response
	resp, ok := httpErr.Message.(ErrorResponse)
	if !ok {
		t.Fatalf("expected ErrorResponse, got %T", httpErr.Message)
	}

	if resp.Code != "AUTH_MISSING" {
		t.Errorf("expected error code AUTH_MISSING, got %s", resp.Code)
	}
}

// TestAuthMiddleware_InvalidHeaderFormat tests that invalid header format returns 401
func TestAuthMiddleware_InvalidHeaderFormat(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := NewAuthMiddleware("https://issuer.example.com", "test-audience", logger)

	e := echo.New()
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// Test with invalid format (no Bearer prefix)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidToken")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}

	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", httpErr.Code)
	}
}

// TestAuthMiddleware_MissingBearerPrefix tests that missing Bearer prefix returns 401
func TestAuthMiddleware_MissingBearerPrefix(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := NewAuthMiddleware("https://issuer.example.com", "test-audience", logger)

	e := echo.New()
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", " ") // Just whitespace
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}

	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", httpErr.Code)
	}
}

// TestRateLimiter_BasicFunctionality tests basic rate limiting
func TestRateLimiter_BasicFunctionality(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := NewRateLimiterMiddleware(10, 10, logger) // High capacity for testing

	e := echo.New()
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// Make 5 requests - should all succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		if err != nil {
			t.Errorf("request %d failed: %v", i+1, err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("request %d expected status 200, got %d", i+1, rec.Code)
		}
	}
}

// TestRateLimiter_TenantIsolation tests that different tenants have separate buckets
func TestRateLimiter_TenantIsolation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := NewRateLimiterMiddleware(2, 1, logger) // Very low capacity

	e := echo.New()
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// Make 2 requests for tenant-1 - should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(string(tenantIDKey), "tenant-1")

		err := handler(c)
		if err != nil {
			t.Errorf("tenant-1 request %d failed: %v", i+1, err)
		}
	}

	// Make 2 requests for tenant-2 - should also succeed (different bucket)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", "tenant-2")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(string(tenantIDKey), "tenant-2")

		err := handler(c)
		if err != nil {
			t.Errorf("tenant-2 request %d failed: %v", i+1, err)
		}
	}
}

// TestRateLimiter_DefaultTenant tests default tenant when no auth
func TestRateLimiter_DefaultTenant(t *testing.T) {
	logger := zaptest.NewLogger(t)
	middleware := NewRateLimiterMiddleware(2, 2, logger) // 2 requests

	e := echo.New()
	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		t.Errorf("first request failed: %v", err)
	}

	// Second request should also succeed for default tenant
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err = handler(c2)
	if err != nil {
		t.Errorf("second request failed: %v", err)
	}
}

// TestTokenBucket_Consume tests token bucket consumption
func TestTokenBucket_Consume(t *testing.T) {
	bucket := &TokenBucket{
		tokens:     5,
		capacity:   5,
		refill:     1,
		lastRefill: time.Now(),
	}

	// Consume 5 tokens
	for i := 0; i < 5; i++ {
		if !bucket.Consume() {
			t.Errorf("expected to consume token %d, but failed", i+1)
		}
	}

	// 6th attempt should fail
	if bucket.Consume() {
		t.Error("expected rate limit, but succeeded")
	}
}

// TestTokenBucket_Refill tests token bucket refill over time
func TestTokenBucket_Refill(t *testing.T) {
	bucket := &TokenBucket{
		tokens:     0,
		capacity:   10,
		refill:     10,                                      // 10 tokens per second
		lastRefill: time.Now().Add(-100 * time.Millisecond), // 100ms ago
	}

	// Should have ~1 token after 100ms
	if !bucket.Consume() {
		t.Error("expected token to be available after refill")
	}
}

// TestTokenBucket_RetryAfter tests RetryAfter calculation
func TestTokenBucket_RetryAfter(t *testing.T) {
	bucket := &TokenBucket{
		tokens:     0,
		capacity:   10,
		refill:     1, // 1 token per second
		lastRefill: time.Now(),
	}

	retryAfter := bucket.RetryAfter()
	if retryAfter <= 0 {
		t.Error("expected positive retry-after value")
	}
}

// TestJWKSCache_KeyNotFound tests handling of missing keys
func TestJWKSCache_KeyNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cache := NewJWKSCache("https://issuer.example.com", logger)

	ctx := context.Background()
	_, err := cache.GetKey(ctx, "non-existent-key")

	if err == nil {
		t.Error("expected error for non-existent key")
	}
}

// TestGetAuthClaims tests retrieving auth claims from context
func TestGetAuthClaims(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test when no claims set
	claims, ok := GetAuthClaims(c)
	if ok {
		t.Error("expected false when no claims set")
	}
	_ = claims

	// Test when claims are set
	expectedClaims := AuthClaims{
		Subject:   "test-user",
		TenantID:  "test-tenant",
		Issuer:    "https://issuer.example.com",
		Audience:  "test-audience",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}
	c.Set(string(authClaimsKey), expectedClaims)

	claims, ok = GetAuthClaims(c)
	if !ok {
		t.Error("expected true when claims are set")
	}
	if claims.Subject != expectedClaims.Subject {
		t.Errorf("expected subject %s, got %s", expectedClaims.Subject, claims.Subject)
	}
}

// TestGetTenantID tests retrieving tenant ID from context
func TestGetTenantID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test when no tenant ID set
	tenantID := GetTenantID(c)
	if tenantID != "" {
		t.Error("expected empty string when no tenant ID set")
	}

	// Test when tenant ID is set
	c.Set(string(tenantIDKey), "test-tenant")
	tenantID = GetTenantID(c)
	if tenantID != "test-tenant" {
		t.Errorf("expected tenant ID test-tenant, got %s", tenantID)
	}
}

// TestMiddlewareError_Error tests MiddlewareError Error method
func TestMiddlewareError_Error(t *testing.T) {
	err := &MiddlewareError{
		Err:  ErrAuthMissing,
		Code: "AUTH_MISSING",
		Detail: map[string]any{
			"header": "Authorization",
		},
	}

	expected := "AUTH_MISSING: missing bearer token"
	if err.Error() != expected {
		t.Errorf("expected %s, got %s", expected, err.Error())
	}
}

// TestMiddlewareError_Unwrap tests MiddlewareError Unwrap method
func TestMiddlewareError_Unwrap(t *testing.T) {
	err := &MiddlewareError{
		Err:  ErrAuthMissing,
		Code: "AUTH_MISSING",
	}

	if err.Unwrap() != ErrAuthMissing {
		t.Error("expected ErrAuthMissing from Unwrap")
	}
}
