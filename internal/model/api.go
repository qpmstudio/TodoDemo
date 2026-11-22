package model

// APIResponse is the standard API response wrapper.
type APIResponse struct {
	Data  interface{} `json:"data"`
	Error *APIError   `json:"error"`
}

// APIError represents an API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`

	// VerifyURL points at the ZITADEL page where the user can finish email
	// verification. It is only set on EMAIL_NOT_VERIFIED errors so existing
	// accounts without a verified email can rescue themselves.
	VerifyURL string `json:"verify_url,omitempty"`

	// VerificationEmailResent reports whether a fresh verification email was
	// sent while rejecting the login. The pointer keeps an explicit false in
	// the payload: a failure to resend must never be silent.
	VerificationEmailResent *bool `json:"verification_email_resent,omitempty"`
}

// Error codes.
const (
	ErrCodeUnauthorized         = "UNAUTHORIZED"
	ErrCodeConflict             = "CONFLICT"
	ErrCodeNotFound             = "NOT_FOUND"
	ErrCodeValidationError      = "VALIDATION_ERROR"
	ErrCodeInternalError        = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable   = "SERVICE_UNAVAILABLE"
	ErrCodeOAuthStateInvalid    = "OAUTH_STATE_INVALID"
	ErrCodeOAuthCodeInvalid     = "OAUTH_CODE_INVALID"
	ErrCodeOAuthIDTokenInvalid  = "OAUTH_ID_TOKEN_INVALID"
	ErrCodeAccountEmailConflict = "ACCOUNT_EMAIL_CONFLICT"
	ErrCodeEmailNotVerified     = "EMAIL_NOT_VERIFIED"
	ErrCodeRegistrationMoved    = "REGISTRATION_MOVED"
	ErrCodeZitadelNotConfigured = "ZITADEL_NOT_CONFIGURED"
	ErrCodeUpstreamUnavailable  = "UPSTREAM_UNAVAILABLE"
)
