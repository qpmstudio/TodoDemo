package zitadel

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ZITADEL's OIDC endpoints are part of its API contract, so they are derived
// from the issuer instead of being discovered on every login. This keeps the
// login path to a single token request in the happy case.
const (
	pathAuthorize  = "/oauth/v2/authorize"
	pathToken      = "/oauth/v2/token"
	pathUserInfo   = "/oidc/v1/userinfo"
	pathJWKS       = "/oauth/v2/keys"
	pathEndSession = "/oidc/v1/end_session"
	// The Auth v1 API lets a signed-in user trigger their own verification
	// mail; the Admin API alternative would need a user.write admin token.
	pathResendEmailVerification = "/auth/v1/users/me/email/_resend_verification"
	jwksCacheTTL                = time.Hour
	maxResponseSize             = 1 << 20
)

// Sentinel errors let the handler map failures to stable HTTP statuses without
// leaking upstream messages to the browser.
var (
	// ErrCodeRejected means ZITADEL rejected the authorization code.
	ErrCodeRejected = errors.New("zitadel: authorization code rejected")
	// ErrUpstream means ZITADEL was unreachable, answered with 5xx, or refused
	// the client at the token endpoint (401 invalid_client / 429 rate limit).
	// Those are configuration or capacity failures the user cannot correct.
	ErrUpstream = errors.New("zitadel: upstream unavailable")
	// ErrInvalidIDToken covers signature, issuer, audience, time and nonce checks.
	ErrInvalidIDToken = errors.New("zitadel: invalid id token")
)

// Config is the subset of TODO configuration needed to talk to ZITADEL.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// HTTPClient is injectable for tests. A client with a 10s timeout is used
	// when nil.
	HTTPClient *http.Client
	// Now is injectable for tests. time.Now is used when nil.
	Now func() time.Time
}

// Client talks to one ZITADEL instance.
type Client struct {
	cfg Config

	mu        sync.Mutex
	keys      map[string]crypto.PublicKey
	algs      map[string]string
	fetchedAt time.Time
}

// New returns an OIDC client for cfg.
func New(cfg Config) *Client {
	cfg.Issuer = strings.TrimRight(cfg.Issuer, "/")
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Client{cfg: cfg}
}

// AuthCodeURL builds the authorization request URL.
func (c *Client) AuthCodeURL(state, nonce, codeVerifier string) string {
	sum := sha256.Sum256([]byte(codeVerifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {c.cfg.RedirectURL},
		"scope":                 {"openid profile email"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
	}
	return c.issuer() + pathAuthorize + "?" + query.Encode()
}

// EndSessionURL builds an RP-initiated logout URL. postLogoutRedirect must be
// registered on the ZITADEL application.
func (c *Client) EndSessionURL(idTokenHint, postLogoutRedirect string) string {
	query := url.Values{"id_token_hint": {idTokenHint}}
	if postLogoutRedirect != "" {
		query.Set("post_logout_redirect_uri", postLogoutRedirect)
	}
	return c.issuer() + pathEndSession + "?" + query.Encode()
}

// Token is the token endpoint response. Access and refresh tokens are never
// persisted; they only live for the duration of the callback.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// Exchange redeems an authorization code for tokens.
func (c *Client) Exchange(ctx context.Context, code, codeVerifier string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.cfg.RedirectURL},
		"code_verifier": {codeVerifier},
	}
	authHeader := ""
	if c.cfg.ClientSecret != "" {
		// RFC 6749 section 2.3.1: form-encode both parts before base64.
		credentials := url.QueryEscape(c.cfg.ClientID) + ":" + url.QueryEscape(c.cfg.ClientSecret)
		authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
	} else {
		form.Set("client_id", c.cfg.ClientID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.issuer()+pathToken, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: token request: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("%w: read token response: %v", ErrUpstream, err)
	}

	if resp.StatusCode >= http.StatusInternalServerError ||
		resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: token endpoint status %d", ErrUpstream, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oauthErr)
		return nil, &OAuthError{StatusCode: resp.StatusCode, Code: oauthErr.Error, Description: oauthErr.ErrorDescription}
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("%w: decode token response: %v", ErrUpstream, err)
	}
	if token.AccessToken == "" || token.IDToken == "" {
		return nil, fmt.Errorf("%w: token response is missing access_token or id_token", ErrUpstream)
	}
	return &token, nil
}

// OAuthError is an OAuth 2.0 error returned by the token endpoint. It unwraps
// to ErrCodeRejected so callers can use errors.Is.
type OAuthError struct {
	StatusCode  int
	Code        string
	Description string
}

func (e *OAuthError) Error() string {
	return fmt.Sprintf("zitadel: token endpoint returned %d (%s)", e.StatusCode, e.Code)
}

// Unwrap makes errors.Is(err, ErrCodeRejected) work.
func (e *OAuthError) Unwrap() error { return ErrCodeRejected }

// Claims is the subset of id_token/userinfo claims TODO consumes.
type Claims struct {
	jwt.RegisteredClaims

	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Nonce             string `json:"nonce"`
	Azp               string `json:"azp"`
}

// VerifyIDToken validates the signature and the standard claims of an id_token.
func (c *Client) VerifyIDToken(ctx context.Context, rawToken, expectedNonce string) (*Claims, error) {
	if rawToken == "" {
		return nil, fmt.Errorf("%w: empty token", ErrInvalidIDToken)
	}

	if err := c.ensureFreshKeys(ctx); err != nil {
		return nil, err
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA"}),
		jwt.WithIssuer(c.cfg.Issuer),
		jwt.WithAudience(c.cfg.ClientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(60*time.Second),
	)

	claims := &Claims{}
	token, err := parser.ParseWithClaims(rawToken, claims, c.keyFunc(ctx))
	if err != nil {
		// Keep both sentinels in the chain: callers still reject the token as
		// invalid, but a JWKS outage surfaces as ErrUpstream so the handler can
		// answer 502 instead of blaming the user's id_token.
		return nil, fmt.Errorf("%w: %w", ErrInvalidIDToken, err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("%w: token is not valid", ErrInvalidIDToken)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing sub", ErrInvalidIDToken)
	}
	if len(claims.Audience) > 1 && claims.Azp != c.cfg.ClientID {
		return nil, fmt.Errorf("%w: azp does not match client_id", ErrInvalidIDToken)
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", ErrInvalidIDToken)
	}
	return claims, nil
}

// UserInfo fetches claims from the userinfo endpoint. ZITADEL does not always
// put profile/email claims in the id_token, so the handler falls back to this.
func (c *Client) UserInfo(ctx context.Context, accessToken string) (*Claims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.issuer()+pathUserInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: userinfo request: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: userinfo status %d", ErrUpstream, resp.StatusCode)
	}

	var claims Claims
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&claims); err != nil {
		return nil, fmt.Errorf("%w: decode userinfo: %v", ErrUpstream, err)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: userinfo is missing sub", ErrUpstream)
	}
	return &claims, nil
}

// ResendEmailVerification asks ZITADEL to send a fresh verification email to
// the user who just signed in with accessToken. It is best-effort: callers keep
// rejecting an unverified login even when this fails, but they report the
// result so the user knows whether to expect a new mail.
func (c *Client) ResendEmailVerification(ctx context.Context, accessToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.issuer()+pathResendEmailVerification, strings.NewReader("{}"))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: resend verification request: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return fmt.Errorf("%w: resend verification status %d: %s",
			ErrUpstream, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseSize))
	return nil
}

func (c *Client) issuer() string { return c.cfg.Issuer }

func (c *Client) keyFunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("%w: token has no kid", ErrInvalidIDToken)
		}

		if key, alg, ok := c.lookup(kid); ok {
			return keyForAlg(key, alg, token.Method.Alg())
		}

		// Unknown kid: the instance likely rotated its keys. Refresh once and
		// try again before rejecting the token.
		if err := c.refreshKeys(ctx); err != nil && !c.hasKeys() {
			return nil, err
		}
		if key, alg, ok := c.lookup(kid); ok {
			return keyForAlg(key, alg, token.Method.Alg())
		}
		return nil, fmt.Errorf("%w: unknown signing key %q", ErrInvalidIDToken, kid)
	}
}

func keyForAlg(key crypto.PublicKey, jwksAlg, tokenAlg string) (interface{}, error) {
	if jwksAlg != "" && jwksAlg != tokenAlg {
		return nil, fmt.Errorf("%w: signing algorithm mismatch", ErrInvalidIDToken)
	}
	return key, nil
}

func (c *Client) lookup(kid string) (crypto.PublicKey, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key, ok := c.keys[kid]
	return key, c.algs[kid], ok
}

func (c *Client) hasKeys() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.keys) > 0
}

func (c *Client) ensureFreshKeys(ctx context.Context) error {
	c.mu.Lock()
	fresh := !c.fetchedAt.IsZero() && c.cfg.Now().Sub(c.fetchedAt) < jwksCacheTTL && len(c.keys) > 0
	c.mu.Unlock()
	if fresh {
		return nil
	}

	err := c.refreshKeys(ctx)
	if err != nil && !c.hasKeys() {
		return err
	}
	// A stale cache is still usable when the refresh fails; the keyfunc will
	// refresh again on a kid miss.
	return nil
}

func (c *Client) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.issuer()+pathJWKS, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: jwks request: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: jwks status %d", ErrUpstream, resp.StatusCode)
	}

	var set jwkSet
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&set); err != nil {
		return fmt.Errorf("%w: decode jwks: %v", ErrUpstream, err)
	}

	keys := make(map[string]crypto.PublicKey, len(set.Keys))
	algs := make(map[string]string, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kid == "" {
			continue
		}
		pub, err := k.publicKey()
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
		algs[k.Kid] = k.Alg
	}
	if len(keys) == 0 {
		return fmt.Errorf("%w: jwks has no usable keys", ErrUpstream)
	}

	c.mu.Lock()
	c.keys = keys
	c.algs = algs
	c.fetchedAt = c.cfg.Now()
	c.mu.Unlock()
	return nil
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jwk) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		n, err := decodeBigInt(k.N)
		if err != nil {
			return nil, err
		}
		e, err := decodeBigInt(k.E)
		if err != nil {
			return nil, err
		}
		if !e.IsInt64() || e.Int64() <= 0 || e.Int64() > int64(^uint32(0)) {
			return nil, errors.New("invalid RSA exponent")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		curve, err := curveByName(k.Crv)
		if err != nil {
			return nil, err
		}
		x, err := decodeBigInt(k.X)
		if err != nil {
			return nil, err
		}
		y, err := decodeBigInt(k.Y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, errors.New("unsupported OKP curve")
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key size")
		}
		return ed25519.PublicKey(x), nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func decodeBigInt(raw string) (*big.Int, error) {
	if raw == "" {
		return nil, errors.New("empty integer")
	}
	bytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(bytes), nil
}

func curveByName(name string) (elliptic.Curve, error) {
	switch name {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported curve %q", name)
	}
}
