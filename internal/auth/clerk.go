package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/interview-eval/backend/internal/models"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Verifier validates Clerk JWTs using JWKS (shared across HTTP middleware and WebSocket).
type Verifier struct {
	cache   *jwk.Cache
	jwksURL string
}

func NewVerifier(jwksURL string) *Verifier {
	if jwksURL == "" {
		return nil
	}
	cache := jwk.NewCache(context.Background())
	_ = cache.Register(jwksURL)
	return &Verifier{cache: cache, jwksURL: jwksURL}
}

// Parse validates a raw JWT string (without "Bearer " prefix).
func (v *Verifier) Parse(ctx context.Context, token string) (userID string, email string, role models.UserRole, err error) {
	keySet, err := v.cache.Get(ctx, v.jwksURL)
	if err != nil {
		return "", "", "", fmt.Errorf("jwks: %w", err)
	}
	tok, err := jwt.Parse([]byte(token), jwt.WithKeySet(keySet), jwt.WithValidate(true))
	if err != nil {
		return "", "", "", fmt.Errorf("jwt: %w", err)
	}
	userID = tok.Subject()
	if em, ok := tok.Get("email"); ok {
		if s, ok := em.(string); ok {
			email = s
		}
	}
	if r, ok := tok.Get("role"); ok {
		if s, ok := r.(string); ok {
			role = models.UserRole(s)
		} else {
			role = models.RoleCandidate
		}
	} else {
		role = models.RoleCandidate
	}
	return userID, email, role, nil
}

// ParseBearer extracts a Bearer token from the Authorization header and parses it.
func (v *Verifier) ParseBearer(ctx context.Context, authorizationHeader string) (userID string, email string, role models.UserRole, err error) {
	if authorizationHeader == "" {
		return "", "", "", fmt.Errorf("missing authorization header")
	}
	parts := strings.SplitN(authorizationHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", "", "", fmt.Errorf("invalid authorization header format")
	}
	return v.Parse(ctx, parts[1])
}
