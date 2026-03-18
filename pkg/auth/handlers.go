package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HandleLogin redirects to OIDC provider or shows BasicAuth form
func (cfg *Config) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if cfg.Mode == "oidc" {
		// TODO: Phase 1 - redirect to OIDC provider
		// For now, return placeholder
		http.Error(w, "OIDC login not yet implemented", http.StatusNotImplemented)
		return
	}

	if cfg.Mode == "basic" {
		// Show login form
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Login</title></head>
<body>
<h1>Login Required</h1>
<form method="post" action="/auth/login">
<input type="text" name="username" placeholder="Username" required />
<input type="password" name="password" placeholder="Password" required />
<button type="submit">Login</button>
</form>
</body>
</html>`))
		return
	}

	http.Error(w, "not configured", http.StatusBadRequest)
}

// HandleCallback handles OIDC callback (Phase 1 placeholder)
func (cfg *Config) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if cfg.Mode != "oidc" {
		http.Error(w, "invalid", http.StatusBadRequest)
		return
	}

	// TODO: Phase 1 - exchange code for token, validate, set session cookie
	http.Error(w, "OIDC callback not yet implemented", http.StatusNotImplemented)
}

// HandleLogout clears session
func (cfg *Config) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "kompass_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.RequireSecureConnection,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to home or login
	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleUserInfo returns current user info from session
func (cfg *Config) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	// Skip if localhost or no auth
	if cfg.IsLocalhost || cfg.Mode == "none" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"sub": "local"})
		return
	}

	user := r.Context().Value("user")
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"sub": fmt.Sprint(user)})
}

// CreateSessionCookie creates a secure session cookie
func (cfg *Config) CreateSessionCookie(session *SessionData) *http.Cookie {
	encoded, _ := session.MarshalToCookie()
	return &http.Cookie{
		Name:     "kompass_session",
		Value:    encoded,
		Path:     "/",
		Expires:  time.Unix(session.ExpiresAt, 0),
		HttpOnly: true,
		Secure:   cfg.RequireSecureConnection,
		SameSite: http.SameSiteLaxMode,
	}
}
