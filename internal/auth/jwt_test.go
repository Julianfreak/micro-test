package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestNewManagerRequiresCredentialsAndSigningKey(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		secret   string
	}{
		{name: "missing username", password: "password", secret: testSecret},
		{name: "missing password", username: "user", secret: testSecret},
		{name: "short signing key", username: "user", password: "password", secret: "too-short"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewManager(test.username, test.password, test.secret); err == nil {
				t.Fatal("NewManager() error = nil, want configuration error")
			}
		})
	}
}

func TestIssueAndValidateToken(t *testing.T) {
	manager, err := NewManager("operator", "strong-password", testSecret)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	token, err := manager.IssueToken("operator", "strong-password")
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	if err := manager.ValidateToken(token); err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if _, err := manager.IssueToken("operator", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("IssueToken() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestValidateTokenRejectsExpiredAndWrongSubject(t *testing.T) {
	manager, err := NewManager("operator", "strong-password", testSecret)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tests := []struct {
		name   string
		claims jwt.RegisteredClaims
	}{
		{
			name: "expired",
			claims: jwt.RegisteredClaims{
				Issuer:    tokenIssuer,
				Subject:   "operator",
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * TokenLifetime)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-TokenLifetime)),
			},
		},
		{
			name: "wrong subject",
			claims: jwt.RegisteredClaims{
				Issuer:    tokenIssuer,
				Subject:   "another-user",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenLifetime)),
			},
		},
		{
			name: "missing expiration",
			claims: jwt.RegisteredClaims{
				Issuer:   tokenIssuer,
				Subject:  "operator",
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
		},
		{
			name: "wrong issuer",
			claims: jwt.RegisteredClaims{
				Issuer:    "another-service",
				Subject:   "operator",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenLifetime)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, test.claims).SignedString(manager.secret)
			if err != nil {
				t.Fatalf("SignedString() error = %v", err)
			}
			if err := manager.ValidateToken(signed); err == nil {
				t.Fatal("ValidateToken() error = nil, want validation error")
			}
		})
	}
}

func TestMiddlewareProtectsRoutesExceptTokenEndpoint(t *testing.T) {
	manager, err := NewManager("operator", "strong-password", testSecret)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := manager.Middleware(next)
	validToken, err := manager.IssueToken("operator", "strong-password")
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		authHeader string
		wantStatus int
	}{
		{name: "missing token", path: "/health/live", wantStatus: http.StatusUnauthorized},
		{name: "token route is public", path: "/auth/token", wantStatus: http.StatusNoContent},
		{name: "session route is public", path: "/auth/session", wantStatus: http.StatusNoContent},
		{name: "logout route is public", path: "/auth/logout", wantStatus: http.StatusNoContent},
		{name: "valid bearer token", path: "/items", authHeader: "Bearer " + validToken, wantStatus: http.StatusNoContent},
		{name: "valid session cookie", path: "/items", wantStatus: http.StatusNoContent},
		{name: "wrong authorization scheme", path: "/items", authHeader: "Basic token", wantStatus: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.authHeader != "" {
				request.Header.Set("Authorization", test.authHeader)
			}
			if test.name == "valid session cookie" {
				request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: validToken})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantStatus == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("unauthorized response missing WWW-Authenticate header")
			}
		})
	}
}
