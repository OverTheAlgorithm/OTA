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

	// Ownership
	GetWithdrawalOwner(ctx context.Context, withdrawalID uuid.UUID) (string, error)
}
