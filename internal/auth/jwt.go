package auth

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const TokenLifetime = 24 * time.Hour

const tokenIssuer = "micro-test"

var ErrInvalidCredentials = errors.New("invalid credentials")

type Manager struct {
	username string
	password string
	secret   []byte
}

func NewManager(username, password, secret string) (*Manager, error) {
	if username == "" {
		return nil, errors.New("JWT_USERNAME must not be empty")
	}
	if password == "" {
		return nil, errors.New("JWT_PASSWORD must not be empty")
	}
	if len([]byte(secret)) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	return &Manager{
		username: username,
		password: password,
		secret:   []byte(secret),
	}, nil
}

func (m *Manager) IssueToken(username, password string) (string, error) {
	validUsername := subtle.ConstantTimeCompare([]byte(username), []byte(m.username))
	validPassword := subtle.ConstantTimeCompare([]byte(password), []byte(m.password))
	if validUsername&validPassword != 1 {
		return "", ErrInvalidCredentials
	}

	now := time.Now().UTC().Truncate(time.Second)
	claims := jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Subject:   m.username,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return signed, nil
}

func (m *Manager) ValidateToken(tokenString string) error {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected JWT signing method")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return fmt.Errorf("validate JWT: %w", err)
	}
	if !token.Valid || claims.Subject != m.username {
		return errors.New("invalid JWT claims")
	}
	return nil
}

func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/token" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || m.ValidateToken(parts[1]) != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if err := json.NewEncoder(w).Encode(struct {
				Error string `json:"error"`
			}{Error: "unauthorized"}); err != nil {
				slog.Error("write unauthorized response", "error", err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
