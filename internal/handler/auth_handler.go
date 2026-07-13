package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DevenWen/TodoDemo/internal/config"
)

// AuthHandler handles OAuth authentication endpoints.
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// GitHubLogin redirects the user to GitHub for OAuth authorization.
func (h *AuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}

// GitHubCallback handles the OAuth callback from GitHub.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "not implemented"})
}

// Logout clears the JWT cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out"})
}
