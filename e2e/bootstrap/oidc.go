// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package bootstrap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceCodeResponse represents the response from Dex's device code endpoint
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse represents the response from Dex's token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// OIDCClient provides OIDC client for device code flow authentication
type OIDCClient struct {
	dexURL       string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	scopes       []string
	testUsername string
	testPassword string
}

// TokenProvider is an interface for obtaining JWT tokens
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

// NewOIDCClient creates a new OIDC client for device code flow
func NewOIDCClient(dexURL, clientID, clientSecret string) *OIDCClient {
	return &OIDCClient{
		dexURL:       strings.TrimSuffix(dexURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		scopes:       []string{"openid", "profile", "email", "groups"},
	}
}

// SetTestCredentials sets test user credentials for automated approval
func (c *OIDCClient) SetTestCredentials(username, password string) {
	c.testUsername = username
	c.testPassword = password
}

// GetToken implements TokenProvider interface using device code flow
func (c *OIDCClient) GetToken(ctx context.Context) (string, error) {
	// Step 1: Request device code
	deviceCode, err := c.requestDeviceCode(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to request device code: %w", err)
	}

	// Step 2: Automate approval (for E2E testing)
	if c.testUsername != "" && c.testPassword != "" {
		if err := c.approveDeviceCode(ctx, deviceCode.UserCode); err != nil {
			return "", fmt.Errorf("failed to approve device code: %w", err)
		}
	} else {
		// In interactive mode, prompt user to visit verification URI
		fmt.Printf("Please visit %s and enter code: %s\n",
			deviceCode.VerificationURI, deviceCode.UserCode)
		fmt.Println("Waiting for approval...")
	}

	// Step 3: Poll for token
	token, err := c.pollForToken(ctx, deviceCode)
	if err != nil {
		return "", fmt.Errorf("failed to poll for token: %w", err)
	}

	return token, nil
}

// requestDeviceCode requests a device code from Dex
func (c *OIDCClient) requestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", c.clientID)
	data.Set("scope", strings.Join(c.scopes, " "))

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.dexURL+"/dex/device/code", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute device code request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed with status %d: %s",
			resp.StatusCode, string(body))
	}

	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}

	return &deviceCode, nil
}

// approveDeviceCode automates approval of device code for E2E testing
func (c *OIDCClient) approveDeviceCode(ctx context.Context, userCode string) error {
	// Dex's device approval endpoint
	approvalURL := c.dexURL + "/dex/device/approve"

	// Prepare form data
	data := url.Values{}
	data.Set("approval", "approve")
	data.Set("user_code", userCode)
	data.Set("login", c.testUsername)
	data.Set("password", c.testPassword)

	req, err := http.NewRequestWithContext(ctx, "POST", approvalURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create approval request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute approval request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device approval failed with status %d: %s",
			resp.StatusCode, string(body))
	}

	return nil
}

// pollForToken polls Dex's token endpoint until user approves
func (c *OIDCClient) pollForToken(ctx context.Context, deviceCode *DeviceCodeResponse) (string, error) {
	interval := time.Duration(deviceCode.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}

	expiry := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			// Check if expired
			if time.Now().After(expiry) {
				return "", fmt.Errorf("device code expired")
			}

			token, err := c.pollTokenOnce(ctx, deviceCode.DeviceCode)
			if err != nil {
				// Check if we should continue polling
				if strings.Contains(err.Error(), "authorization_pending") {
					time.Sleep(interval)
					continue
				}
				return "", err
			}

			return token, nil
		}
	}
}

// pollTokenOnce makes a single token request
func (c *OIDCClient) pollTokenOnce(ctx context.Context, deviceCode string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.dexURL+"/dex/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			return "", fmt.Errorf("%s: %s", errorResp.Error, errorResp.ErrorDescription)
		}
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s",
			resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.IDToken == "" {
		return "", fmt.Errorf("id_token missing from device code response")
	}

	// Return the ID token (JWT) because it carries the requested claims.
	return tokenResp.IDToken, nil
}

// GetTokenWithDeviceCodeFlow is a convenience function that creates a client and gets a token
func GetTokenWithDeviceCodeFlow(ctx context.Context, dexURL, clientID, clientSecret, username, password string) (string, error) {
	client := NewOIDCClient(dexURL, clientID, clientSecret)
	client.SetTestCredentials(username, password)
	return client.GetToken(ctx)
}

// GetTokenWithPasswordGrant obtains a JWT token using OAuth2 password grant flow
func GetTokenWithPasswordGrant(ctx context.Context, dexURL, clientID, clientSecret, username, password string) (string, error) {
	dexURL = strings.TrimSuffix(dexURL, "/")

	// Prepare form data
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", username)
	data.Set("password", password)
	data.Set("scope", "openid email profile groups")

	req, err := http.NewRequestWithContext(ctx, "POST", dexURL+"/dex/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create password grant request: %w", err)
	}

	// Basic authentication: client_id:client_secret
	auth := clientID + ":" + clientSecret
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute password grant request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("password grant request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.IDToken == "" {
		return "", fmt.Errorf("id_token missing from password grant response")
	}

	// Return the ID token (JWT) because it carries the requested claims.
	return tokenResp.IDToken, nil
}
