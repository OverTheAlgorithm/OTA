package collector

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// AIError represents different types of AI client failures
type AIError struct {
	Type    AIErrorType
	Message string
	Cause   error
}

type AIErrorType int

const (
	ErrorTypeNetwork AIErrorType = iota
	ErrorTypeInfrastructure
	ErrorTypeFormat
)

func (e *AIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *AIError) Unwrap() error {
	return e.Cause
}

func (t AIErrorType) String() string {
	switch t {
	case ErrorTypeNetwork:
		return "NetworkError"
	case ErrorTypeInfrastructure:
		return "InfrastructureError"
	case ErrorTypeFormat:
		return "FormatError"
	default:
		return "UnknownError"
	}
}

// ClassifyError determines the error type from a raw error
func ClassifyError(err error) *AIError {
	if err == nil {
		return nil
	}

	// If already an AIError, return it as-is
	var aiErr *AIError
	if errors.As(err, &aiErr) {
		return aiErr
	}

	// Network errors: timeout, connection refused, DNS, etc.
	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &netErr) || errors.As(err, &urlErr) {
		return &AIError{
			Type:    ErrorTypeNetwork,
			Message: "network operation failed",
			Cause:   err,
		}
	}

	// Format errors: unmarshaling, parsing, missing fields
	errMsg := err.Error()
	if containsAny(errMsg, "unmarshal", "parsing", "no output text", "invalid json", "no candidates") {
		return &AIError{
			Type:    ErrorTypeFormat,
			Message: "response format error",
			Cause:   err,
		}
	}

	// Infrastructure errors: HTTP 5xx, rate limits, API errors
	if containsAny(errMsg, "status 5", "api error", "rate limit", "quota exceeded") {
		return &AIError{
			Type:    ErrorTypeInfrastructure,
			Message: "AI service infrastructure error",
			Cause:   err,
		}
	}

	// Default to infrastructure error for unknown cases (safe to retry)
	return &AIError{
		Type:    ErrorTypeInfrastructure,
		Message: "unknown AI client error",
		Cause:   err,
	}
}

// IsRetryable returns true if the error type should be retried
func (e *AIError) IsRetryable() bool {
	return e.Type == ErrorTypeNetwork || e.Type == ErrorTypeInfrastructure
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
