package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Config holds authentication settings
type Config struct {
	Mode                    string // none|oidc|basic
	OIDCIssuerURL           string
	OIDCClientID            string
	OIDCClientSecret        string
	OIDCRedirectURI         string
	BasicAuthUser           string
	BasicAuthHash           string
	RequireSecureConnection bool
	IsLocalhost             bool
}

// LoadConfig reads auth configuration from environment
func LoadConfig(addr string) (*Config, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("KOMPASS_AUTH_MODE")))
	if mode == "" {
		mode = "none"
	}

	// Detect localhost
	isLocalhost := isLocalhostAddr(addr)

	cfg := &Config{
		Mode:                    mode,
		IsLocalhost:             isLocalhost,
		RequireSecureConnection: strings.ToLower(os.Getenv("KOMPASS_REQUIRE_SECURE_CONNECTION")) == "true",
	}

	// Load mode-specific config
	switch mode {
	case "none":
		// No auth needed
	case "oidc":
		cfg.OIDCIssuerURL = strings.TrimSpace(os.Getenv("KOMPASS_OIDC_ISSUER_URL"))
		cfg.OIDCClientID = strings.TrimSpace(os.Getenv("KOMPASS_OIDC_CLIENT_ID"))
		cfg.OIDCClientSecret = strings.TrimSpace(os.Getenv("KOMPASS_OIDC_CLIENT_SECRET"))
		cfg.OIDCRedirectURI = strings.TrimSpace(os.Getenv("KOMPASS_OIDC_REDIRECT_URI"))

		if cfg.OIDCIssuerURL == "" {
			return nil, fmt.Errorf("KOMPASS_AUTH_MODE=oidc but KOMPASS_OIDC_ISSUER_URL not set")
		}
		if cfg.OIDCClientID == "" {
			return nil, fmt.Errorf("KOMPASS_AUTH_MODE=oidc but KOMPASS_OIDC_CLIENT_ID not set")
		}
		if cfg.OIDCClientSecret == "" {
			return nil, fmt.Errorf("KOMPASS_AUTH_MODE=oidc but KOMPASS_OIDC_CLIENT_SECRET not set")
		}
		if cfg.OIDCRedirectURI == "" {
			return nil, fmt.Errorf("KOMPASS_AUTH_MODE=oidc but KOMPASS_OIDC_REDIRECT_URI not set")
		}
	case "basic":
		cfg.BasicAuthUser = strings.TrimSpace(os.Getenv("KOMPASS_BASIC_AUTH_USER"))
		cfg.BasicAuthHash = strings.TrimSpace(os.Getenv("KOMPASS_BASIC_AUTH_HASH"))

		if cfg.BasicAuthUser == "" {
			return nil, fmt.Errorf("KOMPASS_AUTH_MODE=basic but KOMPASS_BASIC_AUTH_USER not set")
		}
		if cfg.BasicAuthHash == "" {
			return nil, fmt.Errorf("KOMPASS_AUTH_MODE=basic but KOMPASS_BASIC_AUTH_HASH not set")
		}
	default:
		return nil, fmt.Errorf("invalid KOMPASS_AUTH_MODE=%q (must be none|oidc|basic)", mode)
	}

	return cfg, nil
}

// isLocalhostAddr checks if address binds to localhost only
func isLocalhostAddr(addr string) bool {
	// Parse host
	host := addr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		host = addr[:idx]
	}

	// Check if it's localhost or 127.0.0.1
	if host == "localhost" || host == "127.0.0.1" || host == "[::1]" {
		return true
	}

	// If empty string, it defaults to 0.0.0.0, not localhost
	return false
}

// Middleware wraps handlers with auth checks
func (cfg *Config) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public auth endpoints
		if strings.HasPrefix(r.URL.Path, "/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		// Localhost bypass
		if cfg.IsLocalhost || cfg.Mode == "none" {
			next.ServeHTTP(w, r)
			return
		}

		// Auth required
		user, err := cfg.authenticate(r)
		if err != nil {
			// Browser: redirect to login
			if isBrowserRequest(r) {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}
			// API: return 401
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Store user in context for downstream handlers
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate checks credentials from session or Basic Auth
func (cfg *Config) authenticate(r *http.Request) (string, error) {
	// Try OIDC session cookie first
	if cfg.Mode == "oidc" {
		cookie, _ := r.Cookie("kompass_session")
		if cookie != nil {
			// For now, assume cookie is valid if present
			// In Phase 2, validate session store
			return cookie.Value, nil
		}
	}

	// Try Basic Auth
	if cfg.Mode == "basic" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Basic ") {
			decoded, err := base64.StdEncoding.DecodeString(auth[6:])
			if err != nil {
				return "", fmt.Errorf("invalid Basic Auth encoding")
			}
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) != 2 {
				return "", fmt.Errorf("invalid Basic Auth format")
			}
			if err := bcrypt.CompareHashAndPassword(
				[]byte(cfg.BasicAuthHash),
				[]byte(parts[1]),
			); err != nil {
				return "", fmt.Errorf("invalid credentials")
			}
			if parts[0] != cfg.BasicAuthUser {
				return "", fmt.Errorf("invalid username")
			}
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("no valid auth")
}

// isBrowserRequest checks if request is from a web browser
func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

// SessionData holds OIDC session info
type SessionData struct {
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	Groups    []string `json:"groups,omitempty"`
	ExpiresAt int64    `json:"exp"`
}

// UnmarshalFromCookie decodes session cookie
func (s *SessionData) UnmarshalFromCookie(data string) error {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, s)
}

// MarshalToCookie encodes session cookie
func (s *SessionData) MarshalToCookie() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// LogStartup logs configuration at startup
func (cfg *Config) LogStartup(addr string) {
	slog.Info(
		"auth configured",
		"mode", cfg.Mode,
		"localhost", cfg.IsLocalhost,
		"addr", addr,
	)
	if cfg.Mode == "oidc" && !cfg.RequireSecureConnection {
		slog.Warn("OIDC without KOMPASS_REQUIRE_SECURE_CONNECTION=true - not recommended", "mode", "oidc")
	}
}
