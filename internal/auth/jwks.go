package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key type: OKP (EdDSA), RSA, EC
	Kid string `json:"kid"` // Key ID
	Alg string `json:"alg"` // Algorithm hint
	Use string `json:"use"` // Key use: sig, enc

	// EdDSA (OKP) fields
	Crv string `json:"crv"` // Curve: Ed25519
	X   string `json:"x"`   // Public key (base64url)

	// RSA fields
	N string `json:"n"` // Modulus (base64url)
	E string `json:"e"` // Exponent (base64url)

	// EC fields (also uses Crv and X)
	Y string `json:"y"` // Y coordinate (base64url)

	// X.509 certificate chain (optional)
	X5c []string `json:"x5c,omitempty"`
}

// JWKS represents a JSON Web Key Set as defined in RFC 7517.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWKSProvider fetches and caches JWKS keys from a remote endpoint.
// It supports automatic background refresh and thread-safe key access.
type JWKSProvider struct {
	url             string
	refreshInterval time.Duration
	httpClient      *http.Client

	mu          sync.RWMutex
	keys        map[string]any // kid -> public key (ed25519.PublicKey, *rsa.PublicKey, *ecdsa.PublicKey)
	lastRefresh time.Time
	initialized bool
}

func NewJWKSProvider(url string, refreshInterval time.Duration) *JWKSProvider {
	return &JWKSProvider{
		url:             url,
		refreshInterval: refreshInterval,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		keys: make(map[string]any),
	}
}

func (p *JWKSProvider) Start(ctx context.Context) error {
	if p.refreshInterval <= 0 {
		return fmt.Errorf("invalid JWKS refresh interval: %s (must be > 0)", p.refreshInterval)
	}

	if err := p.refresh(); err != nil {
		return fmt.Errorf("initial JWKS fetch failed: %w", err)
	}
	p.initialized = true

	go func() {
		ticker := time.NewTicker(p.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[JWKS] Background refresh stopped")
				return
			case <-ticker.C:
				if err := p.refresh(); err != nil {
					log.Printf("[JWKS] Background refresh failed: %v", err)
					// Don't clear existing keys on refresh failure
					// Continue using cached keys until next successful refresh
				}
			}
		}
	}()

	return nil
}

func (p *JWKSProvider) GetKey(kid string) (any, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, fmt.Errorf("JWKS provider not initialized")
	}

	key, ok := p.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", kid)
	}
	return key, nil
}

// GetKeyCount returns the number of cached keys.
func (p *JWKSProvider) GetKeyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.keys)
}

func (p *JWKSProvider) LastRefresh() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastRefresh
}

func (p *JWKSProvider) refresh() error {
	log.Printf("[JWKS] Fetching keys from %s", p.url)

	resp, err := p.httpClient.Get(p.url)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Parse all keys
	keys := make(map[string]any)
	for _, jwk := range jwks.Keys {
		if jwk.Kid == "" {
			log.Printf("[JWKS] Skipping key without kid")
			continue
		}

		key, err := parseJWK(jwk)
		if err != nil {
			log.Printf("[JWKS] Skipping key %s: %v", jwk.Kid, err)
			continue
		}
		keys[jwk.Kid] = key
	}

	if len(keys) == 0 {
		return fmt.Errorf("no valid keys found in JWKS")
	}

	// Update cache atomically
	p.mu.Lock()
	p.keys = keys
	p.lastRefresh = time.Now()
	p.mu.Unlock()

	log.Printf("[JWKS] Successfully loaded %d keys", len(keys))
	return nil
}

func parseJWK(jwk JWK) (any, error) {
	switch jwk.Kty {
	case "OKP":
		return parseEdDSAKey(jwk)
	case "RSA":
		return parseRSAKey(jwk)
	case "EC":
		return parseECKey(jwk)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}

func parseEdDSAKey(jwk JWK) (ed25519.PublicKey, error) {
	if jwk.Crv != "Ed25519" {
		return nil, fmt.Errorf("unsupported OKP curve: %s (expected Ed25519)", jwk.Crv)
	}

	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode X: %w", err)
	}

	if len(x) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 key size: got %d, expected %d", len(x), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(x), nil
}

func parseRSAKey(jwk JWK) (*rsa.PublicKey, error) {
	// If X5c is present, parse from certificate
	if len(jwk.X5c) > 0 {
		return parseRSAFromX5c(jwk.X5c[0])
	}

	// Parse from n and e
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{N: n, E: e}, nil
}

func parseRSAFromX5c(certB64 string) (*rsa.PublicKey, error) {
	certDER, err := base64.StdEncoding.DecodeString(certB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode X5c: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		// Try PEM format
		block, _ := pem.Decode(certDER)
		if block != nil {
			cert, err = x509.ParseCertificate(block.Bytes)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
	}

	rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate does not contain RSA key")
	}

	return rsaKey, nil
}

func parseECKey(jwk JWK) (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode X: %w", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Y: %w", err)
	}

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	var curve elliptic.Curve
	switch jwk.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported EC curve: %s", jwk.Crv)
	}

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
}
