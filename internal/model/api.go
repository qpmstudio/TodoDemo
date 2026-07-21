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
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeValidationError = "VALIDATION_ERROR"
	ErrCodeInternalError   = "INTERNAL_ERROR"
)
