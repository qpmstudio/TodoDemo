package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/model"
	"github.com/DevenWen/TodoDemo/internal/repository"
	"github.com/DevenWen/TodoDemo/internal/zitadel"
)

type fakeZitadelStore struct {
	mu    sync.Mutex
	users map[string]*model.User
	calls int
	err   error
}

func newFakeZitadelStore() *fakeZitadelStore {
	return &fakeZitadelStore{users: map[string]*model.User{}}
}

func (f *fakeZitadelStore) UpsertUserByZitadelSub(ctx context.Context, sub, email, displayName string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if user, ok := f.users[sub]; ok {
		if email != "" {
			user.Email = email
		}
		if displayName != "" {
			user.DisplayName = displayName
		}
		return user, nil
	}
	user := &model.User{
		ID:           "local-" + sub,
		Email:        email,
		DisplayName:  displayName,
		AuthProvider: "zitadel",
	}
	f.users[sub] = user
	return user, nil
}

func (f *fakeZitadelStore) savedUsers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.users)
}

func (f *fakeZitadelStore) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeZitadel is an httptest issuer: it serves JWKS, the token endpoint and an
// optional userinfo endpoint, and signs id_tokens with a locally generated key.
type fakeZitadel struct {
	srv    *httptest.Server
	key    *rsa.PrivateKey
	badKey *rsa.PrivateKey
	kid    string

	mu             sync.Mutex
	nonce          string
	expiresIn      time.Duration
	email          string
	name           string
	emailVerified  bool
	omitProfile    bool
	tokenStatus    int
	wrongKey       bool
	tokenCalls     int
	userInfoEmail    string
	userInfoName     string
	userInfoVerified bool
	userInfoCalled   int
}

func newFakeZitadel(t *testing.T) *fakeZitadel {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	badKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate alternate key: %v", err)
	}
	f := &fakeZitadel{
		key:           key,
		badKey:        badKey,
		kid:           "test-kid",
		expiresIn:     5 * time.Minute,
		email:         "zitadel@example.com",
		name:          "ZITADEL User",
		emailVerified:    true,
		userInfoVerified: true,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v2/keys", f.handleJWKS)
	mux.HandleFunc("/oauth/v2/token", f.handleToken)
	mux.HandleFunc("/oidc/v1/userinfo", f.handleUserInfo)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeZitadel) handleJWKS(w http.ResponseWriter, r *http.Request) {
	pub := f.key.PublicKey
	body := map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": f.kid,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func (f *fakeZitadel) handleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenCalls++

	if f.tokenStatus != 0 && f.tokenStatus != http.StatusOK {
		w.WriteHeader(f.tokenStatus)
		fmt.Fprint(w, `{"error":"server_error"}`)
		return
	}
	if r.PostForm.Get("code") != "good-code" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant"}`)
		return
	}
	if r.PostForm.Get("code_verifier") == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_request","error_description":"missing code_verifier"}`)
		return
	}

	now := time.Now()
	claims := &zitadel.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "zitadel-sub-1",
			Issuer:    f.srv.URL,
			Audience:  jwt.ClaimStrings{"todo-zitadel-client"},
			ExpiresAt: jwt.NewNumericDate(now.Add(f.expiresIn)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Nonce:             f.nonce,
		Email:             f.email,
		EmailVerified:     f.emailVerified,
		Name:              f.name,
		PreferredUsername: "zuser",
	}
	if f.omitProfile {
		claims.Email = ""
		claims.EmailVerified = false
		claims.Name = ""
	}

	signingKey := f.key
	if f.wrongKey {
		signingKey = f.badKey
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = f.kid
	raw, err := token.SignedString(signingKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "access-token",
		"id_token":     raw,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

func (f *fakeZitadel) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userInfoCalled++
	if r.Header.Get("Authorization") != "Bearer access-token" || f.userInfoEmail == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sub":            "zitadel-sub-1",
		"email":          f.userInfoEmail,
		"email_verified": f.userInfoVerified,
		"name":           f.userInfoName,
	})
}

func (f *fakeZitadel) setNonce(nonce string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nonce = nonce
}

func (f *fakeZitadel) setTokenStatus(status int)    { f.tokenStatus = status }
func (f *fakeZitadel) setExpiresIn(d time.Duration) { f.expiresIn = d }
func (f *fakeZitadel) setWrongKey(wrong bool)       { f.wrongKey = wrong }

func zitadelConfigFor(f *fakeZitadel) *config.Config {
	return &config.Config{
		GitHubClientID:      "test-client-id",
		GitHubClientSecret:  "test-client-secret",
		JWTSecret:           "test-jwt-secret",
		FrontendURL:         "http://localhost:5173",
		GitHubRedirectURL:   "http://localhost:8080/auth/github/callback",
		ZitadelIssuer:       f.srv.URL,
		ZitadelClientID:     "todo-zitadel-client",
		ZitadelClientSecret: "todo-secret",
		ZitadelRedirectURL:  "http://localhost:8080/auth/zitadel/callback",
	}
}

func newTestZitadelHandler(t *testing.T, f *fakeZitadel) (*ZitadelHandler, *fakeZitadelStore) {
	t.Helper()
	h := NewZitadelHandler(zitadelConfigFor(f))
	store := newFakeZitadelStore()
	h.store = store
	return h, store
}

type loginResult struct {
	rec      *httptest.ResponseRecorder
	cookie   *http.Cookie
	query    url.Values
	location string
}

func doZitadelLogin(t *testing.T, h *ZitadelHandler, target string) loginResult {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	res := rec.Result()
	var stateCookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == zitadelStateCookie {
			stateCookie = c
		}
	}
	location := res.Header.Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect %q: %v", location, err)
	}
	return loginResult{rec: rec, cookie: stateCookie, query: parsed.Query(), location: location}
}

func doZitadelCallback(t *testing.T, h *ZitadelHandler, code, state string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	target := "/auth/zitadel/callback?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

func TestZitadelLoginRedirectsWithPKCE(t *testing.T) {
	f := newFakeZitadel(t)
	h, _ := newTestZitadelHandler(t, f)

	result := doZitadelLogin(t, h, "/auth/zitadel/login?return_to=%2Ftodos")
	if result.rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", result.rec.Code)
	}
	if !strings.HasPrefix(result.location, f.srv.URL+"/oauth/v2/authorize?") {
		t.Errorf("redirect = %q", result.location)
	}
	checks := map[string]string{
		"response_type":         "code",
		"client_id":             "todo-zitadel-client",
		"redirect_uri":          "http://localhost:8080/auth/zitadel/callback",
		"scope":                 "openid profile email",
		"code_challenge_method": "S256",
	}
	for key, want := range checks {
		if got := result.query.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if result.query.Get("state") == "" || result.query.Get("nonce") == "" || result.query.Get("code_challenge") == "" {
		t.Errorf("missing state/nonce/code_challenge: %v", result.query)
	}
	if result.cookie == nil {
		t.Fatal("state cookie missing")
	}
	if !result.cookie.HttpOnly {
		t.Error("state cookie must be HttpOnly")
	}
	if result.cookie.Path != zitadelCookiePath {
		t.Errorf("state cookie path = %q", result.cookie.Path)
	}
	if result.cookie.MaxAge != int(zitadelStateTTL.Seconds()) {
		t.Errorf("state cookie max age = %d", result.cookie.MaxAge)
	}
}

func TestZitadelLoginNotConfigured(t *testing.T) {
	h := NewZitadelHandler(testConfig())
	req := httptest.NewRequest(http.MethodGet, "/auth/zitadel/login", nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := errorCode(t, rec); got != model.ErrCodeZitadelNotConfigured {
		t.Errorf("code = %q", got)
	}
}

func TestZitadelCallbackCreatesThenReusesUser(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)

	for i := 0; i < 2; i++ {
		login := doZitadelLogin(t, h, "/auth/zitadel/login?return_to=%2Ftodos")
		f.setNonce(login.query.Get("nonce"))

		rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
		if rec.Code != http.StatusFound {
			t.Fatalf("callback %d status = %d, body %s", i, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Location"); got != "http://localhost:5173/todos" {
			t.Errorf("redirect = %q", got)
		}

		res := rec.Result()
		var jwtCookie, idTokenCookie *http.Cookie
		for _, c := range res.Cookies() {
			switch c.Name {
			case "jwt":
				jwtCookie = c
			case zitadelIDTokenCookie:
				idTokenCookie = c
			}
		}
		if jwtCookie == nil || jwtCookie.Value == "" {
			t.Fatal("jwt cookie missing")
		}
		if idTokenCookie == nil || idTokenCookie.Value == "" {
			t.Fatal("zitadel_id_token cookie missing")
		}
		if idTokenCookie.Path != zitadelCookiePath {
			t.Errorf("id_token cookie path = %q", idTokenCookie.Path)
		}
	}

	if store.callCount() != 2 {
		t.Errorf("upsert calls = %d, want 2 (one per login)", store.callCount())
	}
	if store.savedUsers() != 1 {
		t.Errorf("stored users = %d, want 1", store.savedUsers())
	}
	store.mu.Lock()
	user := store.users["zitadel-sub-1"]
	store.mu.Unlock()
	if user == nil || user.Email != "zitadel@example.com" || user.DisplayName != "ZITADEL User" {
		t.Errorf("stored user = %+v", user)
	}
}

func TestZitadelCallbackStateErrors(t *testing.T) {
	f := newFakeZitadel(t)
	h, _ := newTestZitadelHandler(t, f)
	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	t.Run("missing cookie", func(t *testing.T) {
		rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), nil)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != model.ErrCodeOAuthStateInvalid {
			t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
		}
	})

	t.Run("tampered signature", func(t *testing.T) {
		tampered := &http.Cookie{Name: zitadelStateCookie, Value: login.cookie.Value + "x", Path: zitadelCookiePath}
		rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), tampered)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != model.ErrCodeOAuthStateInvalid {
			t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		rec := doZitadelCallback(t, h, "good-code", "not-the-state", login.cookie)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != model.ErrCodeOAuthStateInvalid {
			t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
		}
	})

	t.Run("expired cookie", func(t *testing.T) {
		h.now = func() time.Time { return time.Now().Add(zitadelStateTTL + time.Minute) }
		defer func() { h.now = time.Now }()
		rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != model.ErrCodeOAuthStateInvalid {
			t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
		}
	})
}

func TestZitadelCallbackTokenErrors(t *testing.T) {
	t.Run("rejected code", func(t *testing.T) {
		f := newFakeZitadel(t)
		h, _ := newTestZitadelHandler(t, f)
		login := doZitadelLogin(t, h, "/auth/zitadel/login")
		f.setNonce(login.query.Get("nonce"))

		rec := doZitadelCallback(t, h, "bad-code", login.query.Get("state"), login.cookie)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != model.ErrCodeOAuthCodeInvalid {
			t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
		}
	})

	t.Run("upstream 500", func(t *testing.T) {
		f := newFakeZitadel(t)
		h, _ := newTestZitadelHandler(t, f)
		login := doZitadelLogin(t, h, "/auth/zitadel/login")
		f.setTokenStatus(http.StatusInternalServerError)

		rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
		if rec.Code != http.StatusBadGateway || errorCode(t, rec) != model.ErrCodeUpstreamUnavailable {
			t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
		}
	})
}

func TestZitadelCallbackIDTokenErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *fakeZitadel)
	}{
		{"nonce mismatch", func(f *fakeZitadel) { f.setNonce("wrong-nonce") }},
		{"expired", func(f *fakeZitadel) { f.setExpiresIn(-10 * time.Minute) }},
		{"bad signature", func(f *fakeZitadel) { f.setWrongKey(true) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeZitadel(t)
			h, _ := newTestZitadelHandler(t, f)
			login := doZitadelLogin(t, h, "/auth/zitadel/login")
			f.setNonce(login.query.Get("nonce"))
			tt.setup(f)

			rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != model.ErrCodeOAuthIDTokenInvalid {
				t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
			}
		})
	}
}

func TestZitadelCallbackUserCancelled(t *testing.T) {
	f := newFakeZitadel(t)
	h, _ := newTestZitadelHandler(t, f)

	req := httptest.NewRequest(http.MethodGet, "/auth/zitadel/callback?error=access_denied&state=x", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "http://localhost:5173/login?error=zitadel" {
		t.Errorf("redirect = %q", got)
	}
}

func TestZitadelCallbackEmailConflict(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)
	store.err = repository.ErrEmailConflict

	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
	if rec.Code != http.StatusConflict || errorCode(t, rec) != model.ErrCodeAccountEmailConflict {
		t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
	}
}

func TestZitadelCallbackFallsBackToUserInfo(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)
	f.omitProfile = true
	f.userInfoEmail = "info@example.com"
	f.userInfoName = "Info User"

	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	store.mu.Lock()
	user := store.users["zitadel-sub-1"]
	store.mu.Unlock()
	if user == nil || user.Email != "info@example.com" || user.DisplayName != "Info User" {
		t.Errorf("stored user = %+v", user)
	}
	if f.userInfoCalled == 0 {
		t.Error("userinfo was not called")
	}
}

func noJWTCookie(res *http.Response) bool {
	for _, c := range res.Cookies() {
		if c.Name == "jwt" && c.Value != "" {
			return false
		}
	}
	return true
}

func TestZitadelCallbackRejectsUnverifiedEmail(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)
	f.emailVerified = false

	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body %s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec); got != model.ErrCodeEmailNotVerified {
		t.Errorf("code = %q, want %q", got, model.ErrCodeEmailNotVerified)
	}
	if got := store.savedUsers(); got != 0 {
		t.Errorf("saved users = %d, want 0", got)
	}
	if got := store.callCount(); got != 0 {
		t.Errorf("upsert calls = %d, want 0", got)
	}
	if !noJWTCookie(rec.Result()) {
		t.Error("unverified login must not set a jwt cookie")
	}
}

func TestZitadelCallbackRejectsMissingEmailVerified(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)
	// No profile/email/email_verified in the id_token and no userinfo fallback:
	// the claim is missing, so the login must be rejected.
	f.omitProfile = true

	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body %s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec); got != model.ErrCodeEmailNotVerified {
		t.Errorf("code = %q, want %q", got, model.ErrCodeEmailNotVerified)
	}
	if got := store.savedUsers(); got != 0 {
		t.Errorf("saved users = %d, want 0", got)
	}
	if !noJWTCookie(rec.Result()) {
		t.Error("unverified login must not set a jwt cookie")
	}
}

func TestZitadelCallbackRejectsUserInfoUnverified(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)
	f.emailVerified = false
	f.userInfoEmail = "pending@example.com"
	f.userInfoVerified = false

	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
	if rec.Code != http.StatusForbidden || errorCode(t, rec) != model.ErrCodeEmailNotVerified {
		t.Fatalf("status = %d, code = %q", rec.Code, errorCode(t, rec))
	}
	if got := store.savedUsers(); got != 0 {
		t.Errorf("saved users = %d, want 0", got)
	}
}

func TestZitadelCallbackUnverifiedBrowserRedirectsToLogin(t *testing.T) {
	f := newFakeZitadel(t)
	h, store := newTestZitadelHandler(t, f)
	f.emailVerified = false

	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	target := "/auth/zitadel/callback?code=good-code&state=" + url.QueryEscape(login.query.Get("state"))
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.AddCookie(login.cookie)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302, body %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "http://localhost:5173/login?error=email_not_verified" {
		t.Errorf("redirect = %q", got)
	}
	if got := store.savedUsers(); got != 0 {
		t.Errorf("saved users = %d, want 0", got)
	}
	if !noJWTCookie(rec.Result()) {
		t.Error("unverified login must not set a jwt cookie")
	}
}

func TestZitadelReturnToSanitized(t *testing.T) {
	tests := []struct {
		returnTo string
		want     string
	}{
		{"//evil.example.com", "/todos"},
		{"https://evil.example.com", "/todos"},
		{"/settings", "/settings"},
	}
	for _, tt := range tests {
		t.Run(tt.returnTo, func(t *testing.T) {
			f := newFakeZitadel(t)
			h, _ := newTestZitadelHandler(t, f)
			login := doZitadelLogin(t, h, "/auth/zitadel/login?return_to="+url.QueryEscape(tt.returnTo))
			f.setNonce(login.query.Get("nonce"))

			rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != "http://localhost:5173"+tt.want {
				t.Errorf("redirect = %q, want %q", got, "http://localhost:5173"+tt.want)
			}
		})
	}
}

func TestZitadelLogout(t *testing.T) {
	f := newFakeZitadel(t)
	h, _ := newTestZitadelHandler(t, f)

	t.Run("local only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/zitadel/logout", nil)
		req.AddCookie(&http.Cookie{Name: "jwt", Value: "token"})
		rec := httptest.NewRecorder()
		h.Logout(rec, req)

		if rec.Code != http.StatusFound || rec.Header().Get("Location") != "http://localhost:5173/login" {
			t.Fatalf("status = %d, location = %q", rec.Code, rec.Header().Get("Location"))
		}
		found := false
		for _, c := range rec.Result().Cookies() {
			if c.Name == "jwt" && c.MaxAge < 0 {
				found = true
			}
		}
		if !found {
			t.Error("jwt cookie was not cleared")
		}
	})

	t.Run("sso redirects to end_session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/zitadel/logout?sso=1", nil)
		req.AddCookie(&http.Cookie{Name: zitadelIDTokenCookie, Value: "id-token-1"})
		rec := httptest.NewRecorder()
		h.Logout(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d", rec.Code)
		}
		location := rec.Header().Get("Location")
		if !strings.HasPrefix(location, f.srv.URL+"/oidc/v1/end_session?") {
			t.Fatalf("location = %q", location)
		}
		parsed, _ := url.Parse(location)
		if got := parsed.Query().Get("id_token_hint"); got != "id-token-1" {
			t.Errorf("id_token_hint = %q", got)
		}
		if got := parsed.Query().Get("post_logout_redirect_uri"); got != "http://localhost:5173/login" {
			t.Errorf("post_logout_redirect_uri = %q", got)
		}
	})

	t.Run("sso without id_token stays local", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/zitadel/logout?sso=1", nil)
		rec := httptest.NewRecorder()
		h.Logout(rec, req)
		if rec.Header().Get("Location") != "http://localhost:5173/login" {
			t.Errorf("location = %q", rec.Header().Get("Location"))
		}
	})
}

func TestZitadelProviders(t *testing.T) {
	f := newFakeZitadel(t)

	enabled, _ := newTestZitadelHandler(t, f)
	rec := httptest.NewRecorder()
	enabled.Providers(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data["github"] != true || body.Data["zitadel"] != true {
		t.Errorf("providers = %v", body.Data)
	}
	wantRegister := f.srv.URL + "/ui/v2/login/register"
	if got := body.Data["zitadel_register_url"]; got != wantRegister {
		t.Errorf("zitadel_register_url = %v, want %q", got, wantRegister)
	}

	disabled := NewZitadelHandler(testConfig())
	rec = httptest.NewRecorder()
	disabled.Providers(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", nil))
	body.Data = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data["github"] != true || body.Data["zitadel"] != false {
		t.Errorf("providers = %v", body.Data)
	}
	if _, ok := body.Data["zitadel_register_url"]; ok {
		t.Errorf("disabled ZITADEL must not advertise a register URL: %v", body.Data)
	}
}

func TestRegisterURLDefaultAndOverride(t *testing.T) {
	cfg := &config.Config{ZitadelIssuer: "https://auth.example.com/"}
	if got := cfg.RegisterURL(); got != "https://auth.example.com/ui/v2/login/register" {
		t.Errorf("default register URL = %q", got)
	}
	cfg.ZitadelRegisterURL = "https://id.example.com/signup"
	if got := cfg.RegisterURL(); got != "https://id.example.com/signup" {
		t.Errorf("override register URL = %q", got)
	}
	if got := (&config.Config{}).RegisterURL(); got != "" {
		t.Errorf("register URL without issuer = %q, want empty", got)
	}
}

func TestZitadelCallbackValidJWT(t *testing.T) {
	f := newFakeZitadel(t)
	h, _ := newTestZitadelHandler(t, f)
	login := doZitadelLogin(t, h, "/auth/zitadel/login")
	f.setNonce(login.query.Get("nonce"))

	rec := doZitadelCallback(t, h, "good-code", login.query.Get("state"), login.cookie)
	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "jwt" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("jwt cookie missing")
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte("test-jwt-secret"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse session JWT: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "local-zitadel-sub-1" {
		t.Errorf("sub = %v", claims["sub"])
	}
}
