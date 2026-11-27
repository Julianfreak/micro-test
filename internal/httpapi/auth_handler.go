package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Julianfreak/micro-test/internal/auth"
)

type TokenIssuer interface {
	IssueToken(username, password string) (string, error)
}

type tokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type sessionResponse struct {
	ExpiresIn int64 `json:"expires_in"`
}

// @Summary      Obtain a JWT access token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials body tokenRequest true "Configured username and password"
// @Success      200 {object} tokenResponse
// @Failure      400 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Router       /auth/token [post]
func NewTokenHandler(issuer TokenIssuer) http.Handler {
	return newCredentialHandler(issuer, false, true)
}

// @Summary      Create a persistent browser session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials body tokenRequest true "Configured username and password"
// @Success      200 {object} sessionResponse
// @Failure      400 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Router       /auth/session [post]
func NewSessionHandler(issuer TokenIssuer, secureCookie bool) http.Handler {
	return newCredentialHandler(issuer, secureCookie, false)
}

func newCredentialHandler(issuer TokenIssuer, secureCookie, includeToken bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var credentials tokenRequest
		if err := decoder.Decode(&credentials); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "request body must contain a single JSON object")
			return
		}

		token, err := issuer.IssueToken(credentials.Username, credentials.Password)
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if err != nil {
			slog.ErrorContext(r.Context(), "issue JWT access token", "error", err)
			writeError(w, http.StatusInternalServerError, "could not issue access token")
			return
		}
		if includeToken {
			writeJSON(w, http.StatusOK, tokenResponse{
				AccessToken: token,
				TokenType:   "Bearer",
				ExpiresIn:   int64(auth.TokenLifetime.Seconds()),
			})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(auth.TokenLifetime.Seconds()),
			Expires:  time.Now().Add(auth.TokenLifetime),
		})
		writeJSON(w, http.StatusOK, sessionResponse{
			ExpiresIn: int64(auth.TokenLifetime.Seconds()),
		})
	})
}

// @Summary      End the browser session
// @Tags         auth
// @Success      204
// @Router       /auth/logout [post]
func NewLogoutHandler(secureCookie bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
		w.WriteHeader(http.StatusNoContent)
	})
}
