// Package apperr defines typed domain errors for business logic.
// Infrastructure errors (DB, network) are NOT wrapped here — they are
// propagated as-is or wrapped with fmt.Errorf for context.
package apperr

import "fmt"

// ── Sentinel errors (use errors.Is) ─────────────────────────────────────────

// ErrUnauthorized is returned when the caller lacks permission for the operation.
var ErrUnauthorized = &sentinelError{"unauthorized"}

// ErrBankAccountRequired is returned when a withdrawal is requested but no
// bank account has been registered.
var ErrBankAccountRequired = &sentinelError{"bank account not registered"}

// ErrInsufficientBalance is returned when the user lacks sufficient coins.
var ErrInsufficientBalance = &sentinelError{"insufficient balance"}

// ErrDailyLimitReached is returned when the user has hit their daily earn cap.
var ErrDailyLimitReached = &sentinelError{"daily limit reached"}

// sentinelError is a simple immutable error value comparable via errors.Is.
type sentinelError struct{ msg string }

func (e *sentinelError) Error() string { return e.msg }

// ── Structured errors (use errors.As) ────────────────────────────────────────

// NotFoundError indicates a requested resource does not exist.
type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found", e.Resource)
}

// NewNotFoundError returns a NotFoundError for the given resource name.
func NewNotFoundError(resource string) *NotFoundError {
	return &NotFoundError{Resource: resource}
}

// ValidationError indicates an invalid field value or missing required input.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewValidationError returns a ValidationError for the given field and message.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// ConflictError indicates a state conflict (e.g. acting on a non-pending withdrawal).
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string { return e.Message }

// NewConflictError returns a ConflictError with the given message.
func NewConflictError(message string) *ConflictError {
	return &ConflictError{Message: message}
}

// MinimumAmountError indicates the requested amount is below the allowed minimum.
type MinimumAmountError struct {
	Minimum int
}

func (e *MinimumAmountError) Error() string {
	return fmt.Sprintf("minimum withdrawal amount is %d", e.Minimum)
}

// NewMinimumAmountError returns a MinimumAmountError for the given minimum.
func NewMinimumAmountError(minimum int) *MinimumAmountError {
	return &MinimumAmountError{Minimum: minimum}
}
