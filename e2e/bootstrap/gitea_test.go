// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGiteaClient_Login(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/users/admin/tokens" {
			t.Errorf("expected path /api/v1/users/admin/tokens, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"sha1":"test-token-123"}`)); err != nil {
			t.Fatalf("could not write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewGiteaClient(server.URL)
	token, err := client.Login(context.Background(), "admin", "password", RequiredScopes)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("expected token test-token-123, got %s", token)
	}
}

func TestGiteaClient_CreateOrganization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/orgs" {
			t.Errorf("expected path /api/v1/orgs, got %s", r.URL.Path)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "token test-token" {
			t.Errorf("expected Authorization header 'token test-token', got %s", authHeader)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewGiteaClient(server.URL)
	client.SetToken("test-token")
	err := client.CreateOrganization(context.Background(), "test-org", "Test org")
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}
}

func TestGiteaClient_CreateRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Could be either /api/v1/orgs/test-org/repos or /api/v1/user/repos
		// We'll accept both
		authHeader := r.Header.Get("Authorization")
		if authHeader != "token test-token" {
			t.Errorf("expected Authorization header 'token test-token', got %s", authHeader)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewGiteaClient(server.URL)
	client.SetToken("test-token")
	err := client.CreateRepository(context.Background(), "test-org", "test-repo", "Test repo", false)
	if err != nil {
		t.Fatalf("CreateRepository failed: %v", err)
	}
}

func TestGiteaClient_CreatePersonalAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/users/testuser/tokens" {
			t.Errorf("expected path /api/v1/users/testuser/tokens, got %s", r.URL.Path)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "token test-token" {
			t.Errorf("expected Authorization header 'token test-token', got %s", authHeader)
		}
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"sha1":"pat-token-456"}`)); err != nil {
			t.Fatalf("could not write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewGiteaClient(server.URL)
	client.SetToken("test-token")
	token, err := client.CreatePersonalAccessToken(context.Background(), "testuser", "test-token", []string{"repo", "write:repo"})
	if err != nil {
		t.Fatalf("CreatePersonalAccessToken failed: %v", err)
	}
	if token != "pat-token-456" {
		t.Errorf("expected token pat-token-456, got %s", token)
	}
}
