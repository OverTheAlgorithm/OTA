package withdrawal

import (
	"time"

	"github.com/google/uuid"
)

// Status constants for withdrawal state transitions.
const (
	StatusPending   = "pending"
	StatusApproved  = "approved"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"
)

// BankAccount holds a user's registered bank information.
type BankAccount struct {
	UserID        string `json:"user_id"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

// Withdrawal is the parent record for a withdrawal request.
type Withdrawal struct {
	ID            uuid.UUID `json:"id"`
	UserID        string    `json:"user_id"`
	Amount        int       `json:"amount"`
	BankName      string    `json:"bank_name"`
	AccountNumber string    `json:"account_number"`
	AccountHolder string    `json:"account_holder"`
	CreatedAt     time.Time `json:"created_at"`
	CurrentStatus string    `json:"current_status"` // latest transition status
}

// Transition represents a single state change event for a withdrawal.
type Transition struct {
	ID           uuid.UUID `json:"id"`
	WithdrawalID uuid.UUID `json:"withdrawal_id"`
	Status       string    `json:"status"`
	Note         string    `json:"note"`
	ActorID      string    `json:"actor_id"`
	ActorName    string    `json:"actor_name,omitempty"` // populated on read
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// WithdrawalDetail includes the withdrawal and all its transitions.
type WithdrawalDetail struct {
	Withdrawal
	Transitions          []Transition `json:"transitions"`
	AdblockDetectedAt    *time.Time   `json:"adblock_detected_at"`
	AdblockNotDetectedAt *time.Time   `json:"adblock_not_detected_at"`
}

// WithdrawalListItem is for admin listings with user info.
type WithdrawalListItem struct {
	Withdrawal
	UserNickname         string     `json:"user_nickname"`
	UserEmail            string     `json:"user_email"`
	AdblockDetectedAt    *time.Time `json:"adblock_detected_at"`
	AdblockNotDetectedAt *time.Time `json:"adblock_not_detected_at"`
}

// ListFilter is used by the admin to filter withdrawal listings.
type ListFilter struct {
	Status string // "" = all, or one of the status constants
	Limit  int
	Offset int
}

// PreCheckInfo contains data needed by the frontend before showing the withdrawal modal.
type PreCheckInfo struct {
	MinWithdrawalAmount  int  `json:"min_withdrawal_amount"`
	WithdrawalUnitAmount int  `json:"withdrawal_unit_amount"`
	CurrentBalance       int  `json:"current_balance"`
	HasBankAccount       bool `json:"has_bank_account"`
}
