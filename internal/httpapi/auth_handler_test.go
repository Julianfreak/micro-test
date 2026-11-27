package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Julianfreak/micro-test/internal/auth"
)

func TestTokenHandlerIssuesBearerToken(t *testing.T) {
	manager, err := auth.NewManager("operator", "strong-password", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"username":"operator","password":"strong-password"}`))
	NewTokenHandler(manager).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
	}
	var result tokenResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if result.AccessToken == "" || result.TokenType != "Bearer" || result.ExpiresIn != 86400 {
		t.Fatalf("unexpected token response: %+v", result)
	}
	if err := manager.ValidateToken(result.AccessToken); err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if cookies := response.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("bearer response cookies = %d, want 0", len(cookies))
	}
}

func TestSessionHandlerIssuesHttpOnlySessionCookie(t *testing.T) {
	manager, err := auth.NewManager("operator", "strong-password", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	handler := NewSessionHandler(manager, true)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"username":"operator","password":"strong-password"}`))
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body)
	}
	var result sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if result.ExpiresIn != 86400 {
		t.Fatalf("unexpected token response: %+v", result)
	}
	if strings.Contains(response.Body.String(), "access_token") {
		t.Fatal("login response must not expose the session token")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != auth.SessionCookieName || !cookie.HttpOnly || !cookie.Secure ||
		cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" ||
		cookie.MaxAge != int(auth.TokenLifetime.Seconds()) {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}
	if err := manager.ValidateToken(cookie.Value); err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
}

func TestLogoutHandlerExpiresSessionCookie(t *testing.T) {
	response := httptest.NewRecorder()
	NewLogoutHandler(true).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.SessionCookieName ||
		!cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].MaxAge != -1 {
		t.Fatalf("unexpected logout cookie: %+v", cookies)
	}
}

func TestTokenHandlerRejectsInvalidCredentialsAndJSON(t *testing.T) {
	manager, err := auth.NewManager("operator", "strong-password", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "invalid credentials", body: `{"username":"operator","password":"wrong"}`, wantStatus: http.StatusUnauthorized},
		{name: "malformed JSON", body: `{"username":`, wantStatus: http.StatusBadRequest},
		{name: "unknown field", body: `{"username":"operator","password":"strong-password","admin":true}`, wantStatus: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(test.body))
			NewTokenHandler(manager).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body)
			}
		})
	}
}
