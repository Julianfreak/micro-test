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

	handler := NewTokenHandler(manager)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"username":"operator","password":"strong-password"}`))
	handler.ServeHTTP(response, request)

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
