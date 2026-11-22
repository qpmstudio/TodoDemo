package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/logging"
	"github.com/DevenWen/TodoDemo/internal/middleware"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/DevenWen/TodoDemo/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
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
	// Without an OAuth app there is nothing to authorize against. Redirecting
	// with an empty client_id sends the browser to a GitHub 404, so fail fast
	// with an actionable error instead.
	if h.cfg.GitHubClientID == "" || h.cfg.GitHubClientSecret == "" {
		logging.FromContext(r.Context()).WarnContext(r.Context(),
			"github oauth login rejected: OAuth app not configured",
			"client_id_set", h.cfg.GitHubClientID != "",
			"client_secret_set", h.cfg.GitHubClientSecret != "",
		)
		writeError(w, r, http.StatusServiceUnavailable, model.ErrCodeServiceUnavailable,
			"GitHub login is not configured", nil)
		return
	}

	state, err := generateRandomState()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to generate state", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
	})

	query := url.Values{
		"client_id":    {h.cfg.GitHubClientID},
		"redirect_uri": {h.cfg.GitHubRedirectURL},
		"scope":        {"read:user"},
		"state":        {state},
	}
	authURL := "https://github.com/login/oauth/authorize?" + query.Encode()

	http.Redirect(w, r, authURL, http.StatusFound)
}

// GitHubCallback handles the OAuth callback from GitHub.
func (h *AuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Missing state cookie", err)
		return
	}

	queryState := r.URL.Query().Get("state")
	if queryState == "" || queryState != stateCookie.Value {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Invalid state parameter", nil)
		return
	}

	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Exchange code for access token
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeValidationError, "Missing authorization code", nil)
		return
	}

	accessToken, err := h.exchangeCodeForToken(code)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to exchange code for token: "+err.Error(), err)
		return
	}

	// Get GitHub user info
	githubUser, err := h.getGitHubUser(accessToken)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to get GitHub user info: "+err.Error(), err)
		return
	}

	// Upsert user in database
	gitHubID := githubUser.ID
	user := &model.User{
		GitHubID:        &gitHubID,
		GitHubLogin:     githubUser.Login,
		GitHubAvatarURL: githubUser.AvatarURL,
		DisplayName:     githubUser.Name,
	}
	if user.DisplayName == "" {
		user.DisplayName = githubUser.Login
	}

	if err := repository.UpsertUser(r.Context(), user); err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to save user", err)
		return
	}

	// Create JWT
	jwtToken, err := h.createJWT(user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create JWT", err)
		return
	}

	// Set JWT cookie
	h.setJWTCookie(w, jwtToken)

	// Redirect to frontend
	http.Redirect(w, r, h.cfg.FrontendURL+"/todos", http.StatusFound)
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register handles email + password registration.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, model.ErrCodeValidationError, "Invalid request body", err)
		return
	}

	// Validate email
	if _, err := mail.ParseAddress(req.Email); err != nil || !strings.Contains(req.Email, "@") {
		writeError(w, r, http.StatusUnprocessableEntity, model.ErrCodeValidationError, "Invalid email address", nil)
		return
	}

	// Validate password
	if len(req.Password) < 8 {
		writeError(w, r, http.StatusUnprocessableEntity, model.ErrCodeValidationError, "Password must be at least 8 characters", nil)
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to process password", err)
		return
	}

	// Extract display name from email (part before @)
	displayName := strings.Split(req.Email, "@")[0]

	// Create user
	user, err := repository.CreateUserByEmail(r.Context(), req.Email, string(hash), displayName)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, r, http.StatusConflict, model.ErrCodeConflict, "Email already registered", err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create user", err)
		return
	}

	// Create JWT and set cookie
	jwtToken, err := h.createJWT(user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create JWT", err)
		return
	}

	h.setJWTCookie(w, jwtToken)

	writeJSON(w, http.StatusCreated, user)
}

// Login handles email + password login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnauthorized, model.ErrCodeUnauthorized, "Invalid email or password", err)
		return
	}

	// Get user by email
	user, err := repository.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, model.ErrCodeUnauthorized, "Invalid email or password", err)
		return
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, r, http.StatusUnauthorized, model.ErrCodeUnauthorized, "Invalid email or password", err)
		return
	}

	// Create JWT and set cookie
	jwtToken, err := h.createJWT(user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create JWT", err)
		return
	}

	h.setJWTCookie(w, jwtToken)

	writeJSON(w, http.StatusOK, user)
}

// setJWTCookie sets the JWT cookie on the response.
func (h *AuthHandler) setJWTCookie(w http.ResponseWriter, token string) {
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

func (h *AuthHandler) exchangeCodeForToken(code string) (string, error) {
	data := url.Values{
		"client_id":     {h.cfg.GitHubClientID},
		"client_secret": {h.cfg.GitHubClientSecret},
		"code":          {code},
		"redirect_uri":  {h.cfg.GitHubRedirectURL},
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("github oauth error: %s - %s", result.Error, result.ErrorDescription)
	}

	return result.AccessToken, nil
}

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func (h *AuthHandler) getGitHubUser(accessToken string) (*githubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (h *AuthHandler) createJWT(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"login": user.GitHubLogin,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Helper functions for writing responses
// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
