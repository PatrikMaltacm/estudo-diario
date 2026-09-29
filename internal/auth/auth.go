package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const TokenTTL = 24 * time.Hour

var errInvalidToken = errors.New("token inválido")

type contextKey string

const userIDKey contextKey = "userID"

// DummyHash é usado para comparação constante de tempo em logins inválidos,
// prevenindo ataques de timing.
var DummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), 12)

// GenerateToken cria um JWT assinado com HS256 para o userID fornecido.
func GenerateToken(secret []byte, userID string) (string, error) {
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParseToken valida o JWT e devolve o userID (Subject) ou erro.
func ParseToken(secret []byte, tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !token.Valid {
		return "", errInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", errInvalidToken
	}

	return claims.Subject, nil
}

// RequireAuth é um middleware que valida o Bearer token e injeta o userID no contexto.
func RequireAuth(secret []byte, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Token de acesso ausente", http.StatusUnauthorized)
			return
		}

		userID, err := ParseToken(secret, token)
		if err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Token inválido ou expirado", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// UserIDFromContext extrai o userID injetado pelo middleware RequireAuth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && id != ""
}
