package withdrawal

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence operations for withdrawals.
type Repository interface {
	// Bank account
	GetBankAccount(ctx context.Context, userID string) (*BankAccount, error)
	UpsertBankAccount(ctx context.Context, account BankAccount) error

	// Withdrawal lifecycle
	// CreateWithdrawalWithDeduction atomically deducts coins and creates the withdrawal
	// in a single DB transaction. Uses SELECT FOR UPDATE on user_points to prevent
	// TOCTOU race conditions (double-spend).
	CreateWithdrawalWithDeduction(ctx context.Context, w Withdrawal, actorID string, amount int) (uuid.UUID, error)
	CreateWithdrawal(ctx context.Context, w Withdrawal, actorID string) (uuid.UUID, error)
	GetByID(ctx context.Context, id uuid.UUID) (*WithdrawalDetail, error)
	GetByUser(ctx context.Context, userID string, limit, offset int) ([]WithdrawalDetail, bool, error)

	// Admin listing
	ListAll(ctx context.Context, filter ListFilter) ([]WithdrawalListItem, int, error)

	// Transitions
	AddTransition(ctx context.Context, withdrawalID uuid.UUID, status, note, actorID string) error
	GetLatestStatus(ctx context.Context, withdrawalID uuid.UUID) (string, error)
	GetTransitionByID(ctx context.Context, transitionID uuid.UUID) (*Transition, error)
	UpdateTransitionNote(ctx context.Context, transitionID uuid.UUID, note string) error

	// CancelWithdrawalAtomic atomically verifies the withdrawal is pending,
	// inserts a cancelled transition, and restores coins — all in one transaction.
	CancelWithdrawalAtomic(ctx context.Context, withdrawalID uuid.UUID, actorID string) (int, string, error)

	// RejectWithdrawalAtomic atomically verifies the withdrawal is pending,
	// inserts a rejected transition with a note, and restores coins — all in one transaction.
	RejectWithdrawalAtomic(ctx context.Context, withdrawalID uuid.UUID, actorID, note string) (int, string, error)

	// Ownership
	GetWithdrawalOwner(ctx context.Context, withdrawalID uuid.UUID) (string, error)

	// HasPendingWithdrawals checks if a user has any pending withdrawal requests.
	HasPendingWithdrawals(ctx context.Context, userID string) (bool, error)
}
