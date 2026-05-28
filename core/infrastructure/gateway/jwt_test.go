package gateway

import (
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/require"

    dgw "github.com/ElioNeto/vyx/core/domain/gateway"
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

func TestJWTValidator_Validate_Issuer(t *testing.T) {
    t.Parallel()
    secret := []byte("test-secret")
    validator := NewJWTValidatorWithClaims(secret, "myapp", "")

    makeToken := func(claims jwt.MapClaims) string {
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        signed, err := token.SignedString(secret)
        require.NoError(t, err)
        return signed
    }

    t.Run("valid_issuer", func(t *testing.T) {
        t.Parallel()
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "iss": "myapp",
        })
        c, err := validator.Validate(tokenStr)
        require.NoError(t, err)
        require.Equal(t, "user1", c.UserID)
    })

    t.Run("invalid_issuer", func(t *testing.T) {
        t.Parallel()
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "iss": "other-app",
        })
        _, err := validator.Validate(tokenStr)
        require.ErrorIs(t, err, dgw.ErrUnauthorized)
    })

    t.Run("empty_issuer_in_config_skips_check", func(t *testing.T) {
        t.Parallel()
        v := NewJWTValidatorWithClaims(secret, "", "")
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "iss": "anything",
        })
        _, err := v.Validate(tokenStr)
        require.NoError(t, err)
    })
}

func TestJWTValidator_Validate_Audience(t *testing.T) {
    t.Parallel()
    secret := []byte("test-secret")
    validator := NewJWTValidatorWithClaims(secret, "", "api-service")

    makeToken := func(claims jwt.MapClaims) string {
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        signed, err := token.SignedString(secret)
        require.NoError(t, err)
        return signed
    }

    t.Run("valid_audience", func(t *testing.T) {
        t.Parallel()
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "aud": []string{"api-service"},
        })
        c, err := validator.Validate(tokenStr)
        require.NoError(t, err)
        require.Equal(t, "user1", c.UserID)
    })

    t.Run("invalid_audience", func(t *testing.T) {
        t.Parallel()
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "aud": []string{"other-service"},
        })
        _, err := validator.Validate(tokenStr)
        require.ErrorIs(t, err, dgw.ErrUnauthorized)
    })

    t.Run("empty_audience_in_config_skips_check", func(t *testing.T) {
        t.Parallel()
        v := NewJWTValidatorWithClaims(secret, "", "")
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "aud": []string{"anything"},
        })
        _, err := v.Validate(tokenStr)
        require.NoError(t, err)
    })
}

func TestJWTValidator_Validate_NotBefore(t *testing.T) {
    t.Parallel()
    secret := []byte("test-secret")
    validator := NewJWTValidator(secret)

    makeToken := func(claims jwt.MapClaims) string {
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        signed, err := token.SignedString(secret)
        require.NoError(t, err)
        return signed
    }

    t.Run("nbf_in_past", func(t *testing.T) {
        t.Parallel()
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
            "nbf": time.Now().Add(-time.Hour).Unix(),
        })
        c, err := validator.Validate(tokenStr)
        require.NoError(t, err)
        require.Equal(t, "user1", c.UserID)
    })

    t.Run("nbf_in_future", func(t *testing.T) {
        t.Parallel()
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(2 * time.Hour).Unix(),
            "nbf": time.Now().Add(time.Hour).Unix(),
        })
        _, err := validator.Validate(tokenStr)
        require.ErrorIs(t, err, dgw.ErrUnauthorized)
    })

    t.Run("nbf_omitted", func(t *testing.T) {
        t.Parallel()
        // no nbf claim at all — should be accepted
        tokenStr := makeToken(jwt.MapClaims{
            "sub": "user1", "roles": []string{"admin"},
            "exp": time.Now().Add(time.Hour).Unix(),
        })
        c, err := validator.Validate(tokenStr)
        require.NoError(t, err)
        require.Equal(t, "user1", c.UserID)
    })
}

func TestJWTValidator_NewJWTValidator_BackwardCompat(t *testing.T) {
    t.Parallel()
    secret := []byte("test-secret")
    validator := NewJWTValidator(secret)
    require.NotNil(t, validator)
    // should behave like the old constructor — no iss/aud/nbf enforcement
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": "user1", "roles": []string{"admin"},
        "exp": time.Now().Add(time.Hour).Unix(),
        "iss": "anything",
        "aud": []string{"anything"},
    })
    signed, err := token.SignedString(secret)
    require.NoError(t, err)
    c, err := validator.Validate(signed)
    require.NoError(t, err)
    require.Equal(t, "user1", c.UserID)
}
