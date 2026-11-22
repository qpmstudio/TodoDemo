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
	ErrCodeZitadelNotConfigured = "ZITADEL_NOT_CONFIGURED"
	ErrCodeUpstreamUnavailable  = "UPSTREAM_UNAVAILABLE"
)
