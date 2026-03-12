// Package gateway wires the HTTP server and middleware chain.
package gateway

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// JWTValidator implements application/gateway.JWTValidator using golang-jwt.
type JWTValidator struct {
	secret []byte
}

// NewJWTValidator creates a validator that checks HS256 tokens with the given secret.
func NewJWTValidator(secret []byte) *JWTValidator {
	return &JWTValidator{secret: secret}
}

// Validate parses and verifies a JWT, returning the extracted claims.
func (v *JWTValidator) Validate(tokenStr string) (*dgw.Claims, error) {
	type vyxClaims struct {
		UserID string   `json:"sub"`
		Roles  []string `json:"roles"`
		jwt.RegisteredClaims
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

	return &dgw.Claims{
		UserID: c.UserID,
		Roles:  c.Roles,
	}, nil
}
