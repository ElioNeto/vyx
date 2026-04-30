package gateway

import (
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/require"
)

func TestJWTValidator_Validate(t *testing.T) {
    t.Parallel()
    secret := []byte("test-secret")
    validator := NewJWTValidator(secret)

    // helper to create token
    makeToken := func(claims jwt.MapClaims) string {
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        signed, err := token.SignedString(secret)
        require.NoError(t, err)
        return signed
    }

    // valid token
    t.Run("valid", func(t *testing.T) {
        t.Parallel()
        claims := jwt.MapClaims{"sub": "user1", "roles": []string{"admin"}, "exp": time.Now().Add(time.Hour).Unix()}
        tokenStr := makeToken(claims)
        c, err := validator.Validate(tokenStr)
        require.NoError(t, err)
        require.Equal(t, "user1", c.UserID)
        require.ElementsMatch(t, []string{"admin"}, c.Roles)
    })

    // expired token
    t.Run("expired", func(t *testing.T) {
        t.Parallel()
        claims := jwt.MapClaims{"sub": "user1", "roles": []string{"admin"}, "exp": time.Now().Add(-time.Hour).Unix()}
        tokenStr := makeToken(claims)
        _, err := validator.Validate(tokenStr)
        require.Error(t, err)
    })

    // invalid signature
    t.Run("invalid_signature", func(t *testing.T) {
        t.Parallel()
        otherSecret := []byte("other-secret")
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user1", "roles": []string{"admin"}, "exp": time.Now().Add(time.Hour).Unix()})
        signed, err := token.SignedString(otherSecret)
        require.NoError(t, err)
        _, err = validator.Validate(signed)
        require.Error(t, err)
    })

    // missing claims (should succeed with empty fields)
    t.Run("missing_claims", func(t *testing.T) {
        t.Parallel()
        // token without sub/roles but still valid signature
        claims := jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}
        tokenStr := makeToken(claims)
        c, err := validator.Validate(tokenStr)
        require.NoError(t, err)
        require.Empty(t, c.UserID)
        require.Nil(t, c.Roles)
    })
}
