package auth

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Status:  status,
		Message: message,
	})
}

// Common auth error messages
const (
	ErrMissingAuthHeader = "missing authorization header"
	ErrInvalidAuthFormat = "invalid authorization header format, expected: Bearer <token>"
	ErrInvalidToken      = "invalid token"
	ErrTokenExpired      = "token has expired"
	ErrMissingKid        = "missing key id (kid) in token header"
	ErrKeyNotFound       = "signing key not found"
	ErrAlgorithmMismatch = "token algorithm does not match key type"
	ErrInvalidIssuer     = "invalid token issuer"
	ErrInvalidClaims     = "invalid token claims"
	ErrJWKSUnavailable   = "authentication service unavailable"
)
