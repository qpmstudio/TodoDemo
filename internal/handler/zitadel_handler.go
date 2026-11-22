package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/logging"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/DevenWen/TodoDemo/internal/repository"
	"github.com/DevenWen/TodoDemo/internal/zitadel"
)

const (
	zitadelStateCookie   = "zitadel_oauth"
	zitadelIDTokenCookie = "zitadel_id_token"
	zitadelStateTTL      = 10 * time.Minute
	zitadelCookiePath    = "/auth/zitadel"
	defaultReturnTo      = "/todos"
)

// zitadelUserStore is the user persistence the handler depends on. Tests inject
// a fake; production uses repositoryUserStore.
type zitadelUserStore interface {
	GetUserByZitadelSub(ctx context.Context, sub string) (*model.User, error)
	UpsertUserByZitadelSub(ctx context.Context, sub, email, displayName string) (*model.User, error)
	ClaimUnlinkedUserByEmail(ctx context.Context, sub, email, displayName string) (*model.User, int, error)
}

type repositoryUserStore struct{}

func (repositoryUserStore) UpsertUserByZitadelSub(ctx context.Context, sub, email, displayName string) (*model.User, error) {
	return repository.UpsertUserByZitadelSub(ctx, sub, email, displayName)
}

func (repositoryUserStore) GetUserByZitadelSub(ctx context.Context, sub string) (*model.User, error) {
	return repository.GetUserByZitadelSub(ctx, sub)
}

func (repositoryUserStore) ClaimUnlinkedUserByEmail(ctx context.Context, sub, email, displayName string) (*model.User, int, error) {
	return repository.ClaimUnlinkedUserByEmail(ctx, sub, email, displayName)
}

// oidcClient is the protocol surface used by the handler.
type oidcClient interface {
	AuthCodeURL(state, nonce, codeVerifier string) string
	Exchange(ctx context.Context, code, codeVerifier string) (*zitadel.Token, error)
	VerifyIDToken(ctx context.Context, rawToken, expectedNonce string) (*zitadel.Claims, error)
	UserInfo(ctx context.Context, accessToken string) (*zitadel.Claims, error)
	ResendEmailVerification(ctx context.Context, accessToken string) error
	EndSessionURL(idTokenHint, postLogoutRedirect string) string
}

// ZitadelHandler implements the authorization-code + PKCE login. The resulting
// session is TODO's own JWT cookie, exactly like GitHub and password logins.
type ZitadelHandler struct {
	cfg   *config.Config
	oidc  oidcClient
	store zitadelUserStore
	now   func() time.Time
}

// NewZitadelHandler creates a handler backed by the real repository and a real
// OIDC client.
func NewZitadelHandler(cfg *config.Config) *ZitadelHandler {
	return &ZitadelHandler{
		cfg: cfg,
		oidc: zitadel.New(zitadel.Config{
			Issuer:       cfg.ZitadelIssuer,
			ClientID:     cfg.ZitadelClientID,
			ClientSecret: cfg.ZitadelClientSecret,
			RedirectURL:  cfg.ZitadelRedirectURL,
		}),
		store: repositoryUserStore{},
		now:   time.Now,
	}
}

// Login starts the authorization-code flow.
func (h *ZitadelHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.ZitadelEnabled() {
		logging.FromContext(r.Context()).WarnContext(r.Context(),
			"zitadel login rejected: not configured",
			"client_id_set", h.cfg.ZitadelClientID != "",
			"redirect_url_set", h.cfg.ZitadelRedirectURL != "",
		)
		writeError(w, r, http.StatusServiceUnavailable, model.ErrCodeZitadelNotConfigured,
			"ZITADEL login is not configured", nil)
		return
	}

	state, err := generateRandomState()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to generate state", err)
		return
	}
	nonce, err := generateRandomState()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to generate nonce", err)
		return
	}
	verifier, err := generateRandomState()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to generate PKCE verifier", err)
		return
	}

	sealed, err := h.sealState(&zitadelOAuthState{
		State:     state,
		Nonce:     nonce,
		Verifier:  verifier,
		ReturnTo:  sanitizeReturnTo(r.URL.Query().Get("return_to")),
		ExpiresAt: h.now().Add(zitadelStateTTL).Unix(),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create state cookie", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     zitadelStateCookie,
		Value:    sealed,
		Path:     zitadelCookiePath,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(zitadelStateTTL.Seconds()),
	})

	http.Redirect(w, r, h.oidc.AuthCodeURL(state, nonce, verifier), http.StatusFound)
}

// Callback validates the response, links the ZITADEL subject to a local user
// and issues the normal TODO session cookie.
func (h *ZitadelHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// The state cookie is single-use; clear it even on failure.
	h.clearStateCookie(w)

	query := r.URL.Query()
	if oauthErr := query.Get("error"); oauthErr != "" {
		logging.FromContext(r.Context()).WarnContext(r.Context(),
			"zitadel authorization failed", "error", oauthErr, "description", query.Get("error_description"))
		if oauthErr == "access_denied" {
			http.Redirect(w, r, h.cfg.FrontendURL+"/login?error=zitadel", http.StatusFound)
			return
		}
		writeError(w, r, http.StatusBadRequest, model.ErrCodeOAuthCodeInvalid, "ZITADEL authorization failed", nil)
		return
	}

	cookie, err := r.Cookie(zitadelStateCookie)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeOAuthStateInvalid, "Missing state cookie", err)
		return
	}
	state, err := h.openState(cookie.Value)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeOAuthStateInvalid, "Invalid state cookie", err)
		return
	}

	queryState := query.Get("state")
	if subtle.ConstantTimeCompare([]byte(queryState), []byte(state.State)) != 1 {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeOAuthStateInvalid, "Invalid state parameter", nil)
		return
	}

	code := query.Get("code")
	if code == "" {
		writeError(w, r, http.StatusBadRequest, model.ErrCodeOAuthCodeInvalid, "Missing authorization code", nil)
		return
	}

	token, err := h.oidc.Exchange(r.Context(), code, state.Verifier)
	if err != nil {
		if errors.Is(err, zitadel.ErrCodeRejected) {
			writeError(w, r, http.StatusBadRequest, model.ErrCodeOAuthCodeInvalid, "Authorization code rejected", err)
			return
		}
		writeError(w, r, http.StatusBadGateway, model.ErrCodeUpstreamUnavailable, "ZITADEL token endpoint unavailable", err)
		return
	}

	claims, err := h.oidc.VerifyIDToken(r.Context(), token.IDToken, state.Nonce)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, model.ErrCodeOAuthIDTokenInvalid, "Invalid ZITADEL id_token", err)
		return
	}

	email, name, username := zitadelProfileParts(claims)
	verified := claims.EmailVerified
	if !verified || email == "" || name == "" {
		// ZITADEL may omit profile/email from the id_token when an access token
		// is issued. The userinfo endpoint is authoritative for those claims.
		info, infoErr := h.oidc.UserInfo(r.Context(), token.AccessToken)
		if infoErr != nil {
			logging.FromContext(r.Context()).WarnContext(r.Context(),
				"zitadel userinfo unavailable; continuing with id_token claims", "error", infoErr)
		} else {
			infoEmail, infoName, infoUsername := zitadelProfileParts(info)
			if info.EmailVerified && infoEmail != "" {
				verified = true
				if email == "" {
					email = infoEmail
				}
			}
			if name == "" {
				name = infoName
			}
			if username == "" {
				username = infoUsername
			}
		}
	}

	// Second gate: even when ZITADEL's login page lets an unverified account
	// through, TodoDemo must not create a session or a local user for it.
	if !verified || email == "" {
		logger := logging.FromContext(r.Context())
		// Rejecting alone would strand accounts that predate email
		// verification (some are ACTIVE yet unverified), so the rejection
		// carries a verify link and we ask ZITADEL to send a fresh mail using
		// the user's own access token. A resend failure is reported, never
		// hidden, but it never turns into a login.
		resent := false
		if token.AccessToken == "" {
			logger.WarnContext(r.Context(),
				"cannot resend verification email: no access token", "sub", claims.Subject)
		} else if err := h.oidc.ResendEmailVerification(r.Context(), token.AccessToken); err != nil {
			logger.WarnContext(r.Context(),
				"resend verification email failed", "sub", claims.Subject, "error", err)
		} else {
			resent = true
		}
		logger.WarnContext(r.Context(),
			"zitadel login rejected: email is not verified",
			"sub", claims.Subject, "verification_email_resent", resent)

		if wantsHTML(r) {
			// Carry the resend outcome so the login page can tell the user
			// whether to expect a new mail or to open ZITADEL themselves.
			target := h.cfg.FrontendURL + "/login?error=email_not_verified"
			if resent {
				target += "&resent=1"
			} else {
				target += "&resent=0"
			}
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		writeAPIError(w, r, http.StatusForbidden, &model.APIError{
			Code:                    model.ErrCodeEmailNotVerified,
			Message:                 "Email is not verified; open the verification link from ZITADEL and sign in again",
			VerifyURL:               h.cfg.VerifyURL(),
			VerificationEmailResent: &resent,
		}, nil)
		return
	}

	displayName := zitadelDisplayName(name, username, email)
	user, err := h.store.GetUserByZitadelSub(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to load user", err)
		return
	}
	if user == nil {
		user, err = h.adoptOrCreateUser(r, claims.Subject, email, displayName)
		if err != nil {
			var conflict *accountConflictError
			if errors.As(err, &conflict) {
				writeError(w, r, http.StatusConflict, model.ErrCodeAccountEmailConflict, conflict.message, err)
				return
			}
			writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to save user", err)
			return
		}
	}

	jwtToken, err := createJWT(h.cfg, user)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, model.ErrCodeInternalError, "Failed to create JWT", err)
		return
	}
	setJWTCookie(w, jwtToken)

	if token.IDToken != "" && claims.ExpiresAt != nil {
		h.setIDTokenCookie(w, token.IDToken, claims.ExpiresAt.Time)
	}

	http.Redirect(w, r, h.cfg.FrontendURL+state.ReturnTo, http.StatusFound)
}

// accountConflictError marks a login that must be refused because the verified
// email maps to more than one legacy local account.
type accountConflictError struct{ message string }

func (e *accountConflictError) Error() string { return e.message }

// adoptOrCreateUser keeps pre-ZITADEL accounts reachable. The local row is only
// adopted when exactly one unlinked row carries the verified email; an
// ambiguous email is refused instead of silently creating yet another row, and
// only a proven absence of a legacy row falls through to a fresh insert.
func (h *ZitadelHandler) adoptOrCreateUser(r *http.Request, sub, email, displayName string) (*model.User, error) {
	ctx := r.Context()
	claimed, matches, err := h.store.ClaimUnlinkedUserByEmail(ctx, sub, email, displayName)
	if err != nil {
		return nil, err
	}
	switch {
	case matches > 1:
		logging.FromContext(ctx).WarnContext(ctx,
			"zitadel login found ambiguous unlinked local accounts; refusing to adopt", "matches", matches)
		return nil, &accountConflictError{message: "Several local accounts use this email; an administrator must merge them before this sign-in can continue"}
	case claimed != nil:
		logging.FromContext(ctx).InfoContext(ctx,
			"zitadel login adopted an unlinked local account", "user_id", claimed.ID)
		return claimed, nil
	}
	user, err := h.store.UpsertUserByZitadelSub(ctx, sub, email, displayName)
	if err != nil {
		if errors.Is(err, repository.ErrEmailConflict) {
			return nil, &accountConflictError{message: "This email already belongs to another account; sign in with that method instead"}
		}
		return nil, err
	}
	return user, nil
}

// Logout clears the local session. Single sign-out is opt-in with ?sso=1 and
// only works when the id_token cookie is still valid.
func (h *ZitadelHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearCookie := func(name, path string) {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     path,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.secureCookies(),
			SameSite: http.SameSiteLaxMode,
		})
	}
	clearCookie("jwt", "/")

	hint := ""
	if c, err := r.Cookie(zitadelIDTokenCookie); err == nil {
		hint = c.Value
	}
	clearCookie(zitadelIDTokenCookie, zitadelCookiePath)

	if r.URL.Query().Get("sso") == "1" && hint != "" {
		http.Redirect(w, r, h.oidc.EndSessionURL(hint, h.postLogoutURL()), http.StatusFound)
		return
	}
	http.Redirect(w, r, h.cfg.FrontendURL+"/login", http.StatusFound)
}

// Providers reports which login methods are available. It is registered as a
// public endpoint so the login page can hide unconfigured providers. ZITADEL
// is the only entry: TodoDemo no longer offers password or GitHub sign-in.
func (h *ZitadelHandler) Providers(w http.ResponseWriter, r *http.Request) {
	providers := map[string]any{
		"zitadel": h.cfg.ZitadelEnabled(),
	}
	if h.cfg.ZitadelEnabled() {
		providers["zitadel_register_url"] = h.cfg.RegisterURL()
		providers["zitadel_verify_url"] = h.cfg.VerifyURL()
	}
	writeJSON(w, http.StatusOK, providers)
}

// wantsHTML reports whether the request is a browser top-level navigation, so
// login failures can redirect back to the SPA instead of returning JSON.
func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func (h *ZitadelHandler) postLogoutURL() string {
	if h.cfg.ZitadelPostLogoutURL != "" {
		return h.cfg.ZitadelPostLogoutURL
	}
	return h.cfg.FrontendURL + "/login"
}

func (h *ZitadelHandler) secureCookies() bool {
	return strings.HasPrefix(h.cfg.FrontendURL, "https://") || strings.HasPrefix(h.cfg.ZitadelRedirectURL, "https://")
}

func (h *ZitadelHandler) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     zitadelStateCookie,
		Value:    "",
		Path:     zitadelCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *ZitadelHandler) setIDTokenCookie(w http.ResponseWriter, idToken string, expiresAt time.Time) {
	maxAge := int(expiresAt.Sub(h.now()).Seconds())
	if maxAge <= 0 {
		return
	}
	if maxAge > 7*24*60*60 {
		maxAge = 7 * 24 * 60 * 60
	}
	http.SetCookie(w, &http.Cookie{
		Name:     zitadelIDTokenCookie,
		Value:    idToken,
		Path:     zitadelCookiePath,
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

type zitadelOAuthState struct {
	State     string `json:"state"`
	Nonce     string `json:"nonce"`
	Verifier  string `json:"verifier"`
	ReturnTo  string `json:"return_to"`
	ExpiresAt int64  `json:"exp"`
}

func (h *ZitadelHandler) stateKey() []byte {
	mac := hmac.New(sha256.New, []byte(h.cfg.JWTSecret))
	mac.Write([]byte("tododemo/zitadel/oauth-state"))
	return mac.Sum(nil)
}

// sealState signs the state payload with a key derived from JWT_SECRET so the
// browser cannot choose the nonce, verifier or return_to.
func (h *ZitadelHandler) sealState(state *zitadelOAuthState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, h.stateKey())
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (h *ZitadelHandler) openState(raw string) (*zitadelOAuthState, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return nil, errors.New("malformed state cookie")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode state signature: %w", err)
	}
	mac := hmac.New(sha256.New, h.stateKey())
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, errors.New("state signature mismatch")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode state payload: %w", err)
	}
	var state zitadelOAuthState
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state payload: %w", err)
	}
	if state.State == "" || state.Nonce == "" || state.Verifier == "" {
		return nil, errors.New("state cookie is incomplete")
	}
	if h.now().Unix() > state.ExpiresAt {
		return nil, errors.New("state cookie expired")
	}
	return &state, nil
}

// sanitizeReturnTo only allows local absolute paths; anything else falls back
// to the todo list. This prevents an open redirect through return_to.
func sanitizeReturnTo(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return defaultReturnTo
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return defaultReturnTo
	}
	return parsed.Path
}

func zitadelProfileParts(claims *zitadel.Claims) (email, name, username string) {
	if claims.EmailVerified {
		email = claims.Email
	}
	name = claims.Name
	username = claims.PreferredUsername
	return email, name, username
}

func zitadelDisplayName(name, username, email string) string {
	if name != "" {
		return name
	}
	if username != "" {
		return username
	}
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return "ZITADEL User"
}
