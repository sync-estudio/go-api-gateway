package auth

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Validator struct {
	jwks   *JWKSProvider
	issuer string // Expected issuer (optional, empty = no validation)
}

func NewValidator(jwks *JWKSProvider, issuer string) *Validator {
	return &Validator{
		jwks:   jwks,
		issuer: issuer,
	}
}

func (v *Validator) Validate(tokenString string) (*Identity, error) {
	// Parse and validate the token
	token, err := jwt.Parse(tokenString, v.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, fmt.Errorf(ErrInvalidToken)
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidClaims)
	}

	if v.issuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != v.issuer {
			return nil, fmt.Errorf("%s: expected %s, got %s", ErrInvalidIssuer, v.issuer, iss)
		}
	}

	// Build identity from claims
	identity := &Identity{
		UserID: getString(claims, "sub"),
		Email:  getString(claims, "email"),
		Roles:  getStringSlice(claims, "roles"),
		Claims: claims,
		Token:  tokenString,
	}

	return identity, nil
}

func (v *Validator) keyFunc(token *jwt.Token) (any, error) {
	// Get key ID from header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, fmt.Errorf(ErrMissingKid)
	}

	// Get the key from JWKS
	key, err := v.jwks.GetKey(kid)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", ErrKeyNotFound, kid)
	}

	// Validate that the token algorithm matches the key type
	alg := token.Method.Alg()
	if err := validateAlgorithmForKey(alg, key); err != nil {
		return nil, err
	}

	return key, nil
}

func validateAlgorithmForKey(alg string, key any) error {
	switch key.(type) {
	case ed25519.PublicKey:
		if alg != "EdDSA" {
			return fmt.Errorf("%s: expected EdDSA for Ed25519 key, got %s", ErrAlgorithmMismatch, alg)
		}
	case *rsa.PublicKey:
		if !strings.HasPrefix(alg, "RS") && !strings.HasPrefix(alg, "PS") {
			return fmt.Errorf("%s: expected RS*/PS* for RSA key, got %s", ErrAlgorithmMismatch, alg)
		}
	case *ecdsa.PublicKey:
		if !strings.HasPrefix(alg, "ES") {
			return fmt.Errorf("%s: expected ES* for ECDSA key, got %s", ErrAlgorithmMismatch, alg)
		}
	default:
		return fmt.Errorf("unsupported key type: %T", key)
	}
	return nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringSlice(m map[string]any, key string) []string {
	// Try []string first
	if arr, ok := m[key].([]string); ok {
		return arr
	}

	if arr, ok := m[key].([]any); ok {
		result := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}
