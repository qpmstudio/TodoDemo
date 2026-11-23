package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/DevenWen/TodoDemo/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

// AuthHandler keeps the session endpoints. Signing in and creating accounts
// belong to ZITADEL; TodoDemo only mints and clears its own session cookie.
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// setJWTCookie sets the JWT cookie on the response.
func (h *AuthHandler) setJWTCookie(w http.ResponseWriter, token string) {
	setJWTCookie(w, token)
}

// setJWTCookie is shared by the ZITADEL callback and tests so the session
// cookie keeps the same shape everywhere.
func setJWTCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60, // 7 days
	})
}

// Logout clears the JWT cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

// Me returns the current authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())
	user, err := repository.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, model.ErrCodeNotFound, "User not found", err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) createJWT(user *model.User) (string, error) {
	return createJWT(h.cfg, user)
}

// createJWT issues the HS256 session token for a ZITADEL login.
func createJWT(cfg *config.Config, user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"login": user.DisplayName,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
