package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sync.gateway/internal/config"
)

const (
	defaultSessionTTL = 30 * time.Minute
	cookieName        = "gateway_admin_session"
	defaultUIDistDir  = "ui/dist"
)

type ConfigManager interface {
	CurrentConfig() (*config.YAMLConfig, error)
	ApplyAndPersist(cfg *config.YAMLConfig, path string) error
}

type Credentials struct {
	Email         string
	Password      string
	SessionSecret string
	SessionTTL    time.Duration
	Enabled       bool
}

func LoadCredentialsFromEnv() Credentials {
	ttl := defaultSessionTTL
	if rawTTL := strings.TrimSpace(os.Getenv("ADMIN_SESSION_TTL")); rawTTL != "" {
		if parsedTTL, err := time.ParseDuration(rawTTL); err == nil && parsedTTL > 0 {
			ttl = parsedTTL
		}
	}

	creds := Credentials{
		Email:         strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		Password:      os.Getenv("ADMIN_PASSWORD"),
		SessionSecret: strings.TrimSpace(os.Getenv("ADMIN_SESSION_SECRET")),
		SessionTTL:    ttl,
	}

	creds.Enabled = creds.Email != "" && creds.Password != "" && creds.SessionSecret != ""
	return creds
}

type Handler struct {
	manager     ConfigManager
	configPath  string
	creds       Credentials
	uiDistDir   string
	staticFiles http.Handler
}

func NewHandler(manager ConfigManager, configPath string, creds Credentials) *Handler {
	uiDistDir := strings.TrimSpace(os.Getenv("ADMIN_UI_DIST_DIR"))
	if uiDistDir == "" {
		uiDistDir = defaultUIDistDir
	}

	return &Handler{
		manager:     manager,
		configPath:  configPath,
		creds:       creds,
		uiDistDir:   uiDistDir,
		staticFiles: http.StripPrefix("/admin/", http.FileServer(http.Dir(uiDistDir))),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin", h.handleAdminEntry)
	mux.HandleFunc("/admin/api/login", h.handleLogin)
	mux.HandleFunc("/admin/api/logout", h.handleLogout)
	mux.HandleFunc("/admin/api/session", h.handleSession)
	mux.HandleFunc("/admin/api/config", h.handleConfig)
	mux.Handle("/admin/", h)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/admin/") {
		http.NotFound(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/admin/api/") {
		http.NotFound(w, r)
		return
	}

	if r.URL.Path == "/admin/" {
		indexPath := filepath.Join(h.uiDistDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			http.Error(w, "admin UI assets not found; build ui first with `npm run build` in /ui", http.StatusServiceUnavailable)
			return
		}
	}

	h.staticFiles.ServeHTTP(w, r)
}

func (h *Handler) handleAdminEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
}

func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !h.creds.Enabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":       false,
			"authenticated": false,
		})
		return
	}

	email, ok := h.authenticatedEmail(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":       true,
		"authenticated": ok,
		"email":         email,
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !h.creds.Enabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "admin login is disabled; set ADMIN_EMAIL, ADMIN_PASSWORD, ADMIN_SESSION_SECRET",
		})
		return
	}

	if !sameOrigin(r) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "invalid origin"})
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read request body"})
		return
	}

	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON payload"})
		return
	}

	if !credentialsMatch(h.creds.Email, h.creds.Password, payload.Email, payload.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid credentials"})
		return
	}

	expiresAt := time.Now().Add(h.creds.SessionTTL)
	token, err := h.signSessionToken(h.creds.Email, expiresAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isSecureRequest(r),
		MaxAge:   int(h.creds.SessionTTL.Seconds()),
		Expires:  expiresAt,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"email":      h.creds.Email,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if !sameOrigin(r) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "invalid origin"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isSecureRequest(r),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !h.creds.Enabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "admin API is disabled; set ADMIN_EMAIL, ADMIN_PASSWORD, ADMIN_SESSION_SECRET",
		})
		return
	}

	if _, ok := h.authenticatedEmail(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := h.manager.CurrentConfig()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, cfg)
		return

	case http.MethodPut:
		if !sameOrigin(r) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "invalid origin"})
			return
		}

		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read request body"})
			return
		}

		cfg, err := config.ParseJSON(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}

		if err := h.manager.ApplyAndPersist(cfg, h.configPath); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return

	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func (h *Handler) authenticatedEmail(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", false
	}

	email, expiresAt, ok := h.verifySessionToken(cookie.Value)
	if !ok {
		return "", false
	}

	if time.Now().After(expiresAt) {
		return "", false
	}

	if !secureCompare(strings.ToLower(strings.TrimSpace(email)), strings.ToLower(strings.TrimSpace(h.creds.Email))) {
		return "", false
	}

	return email, true
}

func credentialsMatch(expectedEmail, expectedPassword, givenEmail, givenPassword string) bool {
	normalizedExpectedEmail := strings.ToLower(strings.TrimSpace(expectedEmail))
	normalizedGivenEmail := strings.ToLower(strings.TrimSpace(givenEmail))

	return secureCompare(normalizedExpectedEmail, normalizedGivenEmail) && secureCompare(expectedPassword, givenPassword)
}

func secureCompare(expected, actual string) bool {
	expectedBytes := []byte(expected)
	actualBytes := []byte(actual)
	if len(expectedBytes) != len(actualBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

func (h *Handler) signSessionToken(email string, expiresAt time.Time) (string, error) {
	payload := fmt.Sprintf("%s|%d", email, expiresAt.Unix())
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))

	mac := hmac.New(sha256.New, []byte(h.creds.SessionSecret))
	if _, err := mac.Write([]byte(encodedPayload)); err != nil {
		return "", err
	}

	signature := hex.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature, nil
}

func (h *Handler) verifySessionToken(token string) (string, time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", time.Time{}, false
	}

	encodedPayload := parts[0]
	signature := parts[1]

	mac := hmac.New(sha256.New, []byte(h.creds.SessionSecret))
	_, _ = mac.Write([]byte(encodedPayload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !secureCompare(expectedSignature, signature) {
		return "", time.Time{}, false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", time.Time{}, false
	}

	payload := string(payloadBytes)
	segments := strings.Split(payload, "|")
	if len(segments) != 2 {
		return "", time.Time{}, false
	}

	expiresUnix, err := strconv.ParseInt(segments[1], 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}

	expiresAt := time.Unix(expiresUnix, 0)
	return segments[0], expiresAt, true
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if !strings.EqualFold(originURL.Host, r.Host) {
		return false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.ToLower(forwardedProto)
	}

	return strings.EqualFold(originURL.Scheme, scheme)
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
