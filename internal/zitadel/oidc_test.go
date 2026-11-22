package zitadel

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	testKeyOnce sync.Once
	testKey     *rsa.PrivateKey
	testKeyErr  error
)

func signingKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testKeyOnce.Do(func() {
		testKey, testKeyErr = rsa.GenerateKey(rand.Reader, 2048)
	})
	if testKeyErr != nil {
		t.Fatalf("generate RSA key: %v", testKeyErr)
	}
	return testKey
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func jwkFor(kid string, key *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"n":   b64(key.N.Bytes()),
		"e":   b64(big.NewInt(int64(key.E)).Bytes()),
	}
}

func jwksBody(keys ...map[string]string) string {
	raw, _ := json.Marshal(map[string]any{"keys": keys})
	return string(raw)
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims *Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func baseClaims(issuer, audience, nonce string) *Claims {
	now := time.Now()
	return &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "zitadel-user-1",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Email:             "user@example.com",
		EmailVerified:     true,
		PreferredUsername: "user",
		Name:              "Test User",
		Nonce:             nonce,
	}
}

func newServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func testClient(srv *httptest.Server) *Client {
	return New(Config{
		Issuer:       srv.URL,
		ClientID:     "todo-client",
		ClientSecret: "todo-secret",
		RedirectURL:  "http://localhost:8080/auth/zitadel/callback",
	})
}

func TestAuthCodeURL(t *testing.T) {
	client := New(Config{
		Issuer:      "https://auth.example.com/",
		ClientID:    "client-1",
		RedirectURL: "https://todo.example.com/auth/zitadel/callback",
	})

	raw := client.AuthCodeURL("state-1", "nonce-1", "verifier-1")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if parsed.Path != "/oauth/v2/authorize" {
		t.Errorf("path = %q, want /oauth/v2/authorize", parsed.Path)
	}
	if parsed.Host != "auth.example.com" {
		t.Errorf("host = %q, want auth.example.com", parsed.Host)
	}

	q := parsed.Query()
	want := map[string]string{
		"response_type":         "code",
		"client_id":             "client-1",
		"redirect_uri":          "https://todo.example.com/auth/zitadel/callback",
		"scope":                 "openid profile email",
		"state":                 "state-1",
		"nonce":                 "nonce-1",
		"code_challenge_method": "S256",
	}
	for key, value := range want {
		if got := q.Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
	if q.Get("code_challenge") == "" {
		t.Error("code_challenge is empty")
	}
}

func TestEndSessionURL(t *testing.T) {
	client := New(Config{Issuer: "https://auth.example.com"})
	raw := client.EndSessionURL("id-token", "https://todo.example.com/login")
	parsed, _ := url.Parse(raw)
	if parsed.Path != "/oidc/v1/end_session" {
		t.Errorf("path = %q", parsed.Path)
	}
	if got := parsed.Query().Get("id_token_hint"); got != "id-token" {
		t.Errorf("id_token_hint = %q", got)
	}
	if got := parsed.Query().Get("post_logout_redirect_uri"); got != "https://todo.example.com/login" {
		t.Errorf("post_logout_redirect_uri = %q", got)
	}
}

func TestExchange(t *testing.T) {
	var gotForm url.Values
	var gotAuth string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/v2/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotForm = r.PostForm
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"access-1","id_token":"id-1","token_type":"Bearer","expires_in":3600}`)
	})
	client := testClient(newServer(t, handler))

	token, err := client.Exchange(context.Background(), "code-1", "verifier-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if token.AccessToken != "access-1" || token.IDToken != "id-1" {
		t.Errorf("unexpected token: %+v", token)
	}
	if got := gotForm.Get("grant_type"); got != "authorization_code" {
		t.Errorf("grant_type = %q", got)
	}
	if got := gotForm.Get("code"); got != "code-1" {
		t.Errorf("code = %q", got)
	}
	if got := gotForm.Get("redirect_uri"); got != client.cfg.RedirectURL {
		t.Errorf("redirect_uri = %q", got)
	}
	if got := gotForm.Get("code_verifier"); got != "verifier-1" {
		t.Errorf("code_verifier = %q", got)
	}
	if got := gotForm.Get("client_id"); got != "" {
		t.Errorf("client_id must not be in the form when a secret is used, got %q", got)
	}
	if gotAuth == "" {
		t.Error("expected client_secret_basic Authorization header")
	}
}

func TestExchangePublicClient(t *testing.T) {
	var gotForm url.Values
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		fmt.Fprint(w, `{"access_token":"access-1","id_token":"id-1"}`)
	})
	srv := newServer(t, handler)
	client := New(Config{
		Issuer:      srv.URL,
		ClientID:    "public-client",
		RedirectURL: "http://localhost:8080/auth/zitadel/callback",
	})

	if _, err := client.Exchange(context.Background(), "code-1", "verifier-1"); err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if got := gotForm.Get("client_id"); got != "public-client" {
		t.Errorf("client_id = %q", got)
	}
}

func TestExchangeErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{"rejected code", http.StatusBadRequest, `{"error":"invalid_grant"}`, ErrCodeRejected},
		{"invalid client", http.StatusUnauthorized, `{"error":"invalid_client"}`, ErrUpstream},
		{"rate limited", http.StatusTooManyRequests, `{"error":"too_many_requests"}`, ErrUpstream},
		{"upstream 500", http.StatusInternalServerError, `oops`, ErrUpstream},
		{"missing id_token", http.StatusOK, `{"access_token":"a"}`, ErrUpstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			})
			client := testClient(newServer(t, handler))
			_, err := client.Exchange(context.Background(), "code", "verifier")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyIDToken(t *testing.T) {
	key := signingKey(t)
	kid := "key-1"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/v2/keys" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, jwksBody(jwkFor(kid, &key.PublicKey)))
	})
	srv := newServer(t, handler)
	client := testClient(srv)

	valid := signToken(t, key, kid, baseClaims(srv.URL, "todo-client", "nonce-1"))
	if _, err := client.VerifyIDToken(context.Background(), valid, "nonce-1"); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	tests := []struct {
		name   string
		token  string
		nonce  string
		client *Client
	}{
		{
			name:   "nonce mismatch",
			token:  valid,
			nonce:  "other-nonce",
			client: client,
		},
		{
			name:   "wrong issuer",
			token:  signToken(t, key, kid, baseClaims("https://evil.example.com", "todo-client", "nonce-1")),
			nonce:  "nonce-1",
			client: client,
		},
		{
			name:   "wrong audience",
			token:  signToken(t, key, kid, baseClaims(srv.URL, "other-client", "nonce-1")),
			nonce:  "nonce-1",
			client: client,
		},
		{
			name: "expired",
			token: signToken(t, key, kid, func() *Claims {
				c := baseClaims(srv.URL, "todo-client", "nonce-1")
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Minute))
				return c
			}()),
			nonce:  "nonce-1",
			client: client,
		},
		{
			name:   "unknown kid",
			token:  signToken(t, key, "unknown-kid", baseClaims(srv.URL, "todo-client", "nonce-1")),
			nonce:  "nonce-1",
			client: client,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.client.VerifyIDToken(context.Background(), tt.token, tt.nonce); !errors.Is(err, ErrInvalidIDToken) {
				t.Fatalf("err = %v, want ErrInvalidIDToken", err)
			}
		})
	}
}

func TestVerifyIDTokenRefreshesRotatedKid(t *testing.T) {
	key := signingKey(t)
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			// First response has an unrelated key, simulating a stale cache.
			fmt.Fprint(w, jwksBody(jwkFor("old-kid", &key.PublicKey)))
			return
		}
		fmt.Fprint(w, jwksBody(jwkFor("new-kid", &key.PublicKey)))
	})
	srv := newServer(t, handler)
	client := testClient(srv)

	claims := baseClaims(srv.URL, "todo-client", "nonce-1")
	raw := signToken(t, key, "new-kid", claims)

	if _, err := client.VerifyIDToken(context.Background(), raw, "nonce-1"); err != nil {
		t.Fatalf("VerifyIDToken after rotation: %v", err)
	}
	if calls.Load() < 2 {
		t.Errorf("expected a forced JWKS refresh, got %d fetch(es)", calls.Load())
	}
}

func TestUserInfo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-1" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `{"sub":"zitadel-user-1","email":"user@example.com","email_verified":true,"name":"Test User"}`)
	})
	client := testClient(newServer(t, handler))

	claims, err := client.UserInfo(context.Background(), "access-1")
	if err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
	if claims.Subject != "zitadel-user-1" || claims.Email != "user@example.com" || !claims.EmailVerified {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestUserInfoError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	client := testClient(newServer(t, handler))
	if _, err := client.UserInfo(context.Background(), "bad"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}
