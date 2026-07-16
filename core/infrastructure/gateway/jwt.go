// Package gateway wires the HTTP server and middleware chain.
package gateway

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// JWTValidator implements application/gateway.JWTValidator using golang-jwt.
type JWTValidator struct {
	secret   []byte
	issuer   string // expected issuer (optional)
	audience string // expected audience (optional)
}

// NewJWTValidator creates a validator that checks HS256 tokens with the given secret.
// For backward compatibility — does not validate iss, aud, or nbf.
func NewJWTValidator(secret []byte) *JWTValidator {
	return NewJWTValidatorWithClaims(secret, "", "")
}

// NewJWTValidatorWithClaims creates a validator that checks HS256 tokens and
// additionally validates iss (issuer), aud (audience), and nbf (not before)
// when the corresponding values are non-empty.
func NewJWTValidatorWithClaims(secret []byte, issuer, audience string) *JWTValidator {
	return &JWTValidator{
		secret:   secret,
		issuer:   issuer,
		audience: audience,
	}
}

// Validate parses and verifies a JWT, returning the extracted claims.
func (v *JWTValidator) Validate(tokenStr string) (*dgw.Claims, error) {
	type vyxClaims struct {
		UserID string   `json:"sub"`
		Roles  []string `json:"roles"`
		jwt.RegisteredClaims
		// Support both "sub" and "user_id" claim names for compatibility
		UserIDCustom string `json:"user_id"`
	}

	token, err := jwt.ParseWithClaims(tokenStr, &vyxClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.secret, nil
	})
	if err != nil {
		return nil, err
	}

	c, ok := token.Claims.(*vyxClaims)
	if !ok || !token.Valid {
		return nil, errors.New("jwt: invalid token claims")
	}

	// Validate iss (issuer) when an expected issuer is configured.
	if v.issuer != "" && c.Issuer != v.issuer {
		return nil, dgw.ErrUnauthorized
	}

	// Validate aud (audience) when an expected audience is configured.
	if v.audience != "" {
		if !contains(c.Audience, v.audience) {
			return nil, dgw.ErrUnauthorized
		}
	}

	// Validate nbf (not before) — reject tokens that are not yet valid.
	if c.NotBefore != nil && time.Now().Before(c.NotBefore.Time) {
		return nil, dgw.ErrUnauthorized
	}

	// Use "sub" first, fall back to "user_id" custom claim
	userID := c.UserID
	if userID == "" {
		userID = c.UserIDCustom
	}

	return &dgw.Claims{
		UserID: userID,
		Roles:  c.Roles,
	}, nil
}

// contains reports whether a slice of strings contains the target value.
func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
