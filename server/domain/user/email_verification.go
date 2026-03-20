package user

import (
	"context"
	"time"
)

type EmailVerificationCode struct {
	ID        string
	UserID    string
	Email     string
	Code      string
	ExpiresAt time.Time
	Used      bool
	Attempts  int
	CreatedAt time.Time
}

type EmailVerificationRepository interface {
	// CreateCode stores a new verification code
	CreateCode(ctx context.Context, code EmailVerificationCode) error

	// FindLatestPendingCode returns the most recent unexpired, unused code for the user
	// (regardless of code value) so attempts can be checked before matching
	FindLatestPendingCode(ctx context.Context, userID string) (EmailVerificationCode, error)

	// FindValidCode returns a matching, unexpired, unused code for the user
	FindValidCode(ctx context.Context, userID string, code string) (EmailVerificationCode, error)

	// MarkCodeUsed marks a code as used (idempotent)
	MarkCodeUsed(ctx context.Context, codeID string) error

	// IncrementAttempts atomically increments the attempts counter for a code
	IncrementAttempts(ctx context.Context, codeID string) error

	// CountRecentCodes returns the number of codes created for a user in the given duration
	// Used for rate limiting
	CountRecentCodes(ctx context.Context, userID string, since time.Time) (int, error)

	// InvalidatePendingCodes marks all pending (unused, unexpired) codes for a user as used
	// Called after a successful verification so old codes cannot be reused
	InvalidatePendingCodes(ctx context.Context, userID string) error
}
