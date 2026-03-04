package withdrawal

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CoinManager abstracts coin balance operations (implemented by level.Repository).
type CoinManager interface {
	GetUserCoins(ctx context.Context, userID string) (int, error)
	DeductCoins(ctx context.Context, userID string, amount int) error
	RestoreCoins(ctx context.Context, userID string, amount int) error
}

// coinManagerAdapter wraps level.Repository to implement CoinManager.
// Defined here so callers only need to pass the level repo.
type coinManagerAdapter struct {
	getCoins func(ctx context.Context, userID string) (int, error)
	deduct   func(ctx context.Context, userID string, amount int) error
	restore  func(ctx context.Context, userID string, amount int) error
}

func (a *coinManagerAdapter) GetUserCoins(ctx context.Context, userID string) (int, error) {
	return a.getCoins(ctx, userID)
}
func (a *coinManagerAdapter) DeductCoins(ctx context.Context, userID string, amount int) error {
	return a.deduct(ctx, userID, amount)
}
func (a *coinManagerAdapter) RestoreCoins(ctx context.Context, userID string, amount int) error {
	return a.restore(ctx, userID, amount)
}

// NewCoinManager creates a CoinManager from individual function references.
func NewCoinManager(
	getCoins func(ctx context.Context, userID string) (int, error),
	deduct func(ctx context.Context, userID string, amount int) error,
	restore func(ctx context.Context, userID string, amount int) error,
) CoinManager {
	return &coinManagerAdapter{getCoins: getCoins, deduct: deduct, restore: restore}
}

type Service struct {
	repo                Repository
	coinManager         CoinManager
	minWithdrawalAmount int
}

func NewService(repo Repository, coinManager CoinManager, minWithdrawalAmount int) *Service {
	return &Service{
		repo:                repo,
		coinManager:         coinManager,
		minWithdrawalAmount: minWithdrawalAmount,
	}
}

// GetMinWithdrawalAmount returns the configured minimum withdrawal amount.
func (s *Service) GetMinWithdrawalAmount() int {
	return s.minWithdrawalAmount
}

// GetBankAccount returns the user's registered bank account, or nil if none.
func (s *Service) GetBankAccount(ctx context.Context, userID string) (*BankAccount, error) {
	return s.repo.GetBankAccount(ctx, userID)
}

// SaveBankAccount creates or updates the user's bank account info.
func (s *Service) SaveBankAccount(ctx context.Context, account BankAccount) error {
	if strings.TrimSpace(account.BankName) == "" {
		return fmt.Errorf("bank_name is required")
	}
	if strings.TrimSpace(account.AccountNumber) == "" {
		return fmt.Errorf("account_number is required")
	}
	if strings.TrimSpace(account.AccountHolder) == "" {
		return fmt.Errorf("account_holder is required")
	}
	return s.repo.UpsertBankAccount(ctx, account)
}

// RequestWithdrawal creates a new withdrawal request.
// Validates: bank account registered, minimum amount, sufficient balance.
// Deducts coins immediately and creates the withdrawal + pending transition.
func (s *Service) RequestWithdrawal(ctx context.Context, userID string, amount int) (*Withdrawal, error) {
	// 1. Check bank account
	account, err := s.repo.GetBankAccount(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check bank account: %w", err)
	}
	if account == nil {
		return nil, fmt.Errorf("bank account not registered")
	}

	// 2. Validate minimum amount
	if amount < s.minWithdrawalAmount {
		return nil, fmt.Errorf("minimum withdrawal amount is %d", s.minWithdrawalAmount)
	}

	// 3. Check balance
	coins, err := s.coinManager.GetUserCoins(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check balance: %w", err)
	}
	if coins < amount {
		return nil, fmt.Errorf("insufficient coins: have %d, need %d", coins, amount)
	}

	// 4. Deduct coins immediately
	if err := s.coinManager.DeductCoins(ctx, userID, amount); err != nil {
		return nil, fmt.Errorf("deduct coins: %w", err)
	}

	// 5. Create withdrawal + initial pending transition
	w := Withdrawal{
		UserID:        userID,
		Amount:        amount,
		BankName:      account.BankName,
		AccountNumber: account.AccountNumber,
		AccountHolder: account.AccountHolder,
	}
	id, err := s.repo.CreateWithdrawal(ctx, w, userID)
	if err != nil {
		// Attempt to restore coins on failure
		_ = s.coinManager.RestoreCoins(ctx, userID, amount)
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	w.ID = id
	w.CurrentStatus = StatusPending
	return &w, nil
}

// CancelWithdrawal cancels a pending withdrawal and restores coins.
func (s *Service) CancelWithdrawal(ctx context.Context, userID string, withdrawalID uuid.UUID) error {
	// 1. Verify ownership
	owner, err := s.repo.GetWithdrawalOwner(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get withdrawal owner: %w", err)
	}
	if owner != userID {
		return fmt.Errorf("not authorized")
	}

	// 2. Check current status
	status, err := s.repo.GetLatestStatus(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if status != StatusPending {
		return fmt.Errorf("can only cancel pending withdrawals (current: %s)", status)
	}

	// 3. Get withdrawal amount for coin restoration
	detail, err := s.repo.GetByID(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get withdrawal: %w", err)
	}

	// 4. Add cancelled transition
	if err := s.repo.AddTransition(ctx, withdrawalID, StatusCancelled, "", userID); err != nil {
		return fmt.Errorf("add transition: %w", err)
	}

	// 5. Restore coins
	if err := s.coinManager.RestoreCoins(ctx, detail.UserID, detail.Amount); err != nil {
		return fmt.Errorf("restore coins: %w", err)
	}

	return nil
}

// ApproveWithdrawal approves a pending withdrawal (admin action).
func (s *Service) ApproveWithdrawal(ctx context.Context, adminID string, withdrawalID uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("note is required")
	}

	status, err := s.repo.GetLatestStatus(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if status != StatusPending {
		return fmt.Errorf("can only approve pending withdrawals (current: %s)", status)
	}

	return s.repo.AddTransition(ctx, withdrawalID, StatusApproved, note, adminID)
}

// RejectWithdrawal rejects a pending withdrawal (admin action) and restores coins.
func (s *Service) RejectWithdrawal(ctx context.Context, adminID string, withdrawalID uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("rejection reason is required")
	}

	status, err := s.repo.GetLatestStatus(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if status != StatusPending {
		return fmt.Errorf("can only reject pending withdrawals (current: %s)", status)
	}

	// Get withdrawal for coin restoration
	detail, err := s.repo.GetByID(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get withdrawal: %w", err)
	}

	// Add rejected transition
	if err := s.repo.AddTransition(ctx, withdrawalID, StatusRejected, note, adminID); err != nil {
		return fmt.Errorf("add transition: %w", err)
	}

	// Restore coins
	if err := s.coinManager.RestoreCoins(ctx, detail.UserID, detail.Amount); err != nil {
		return fmt.Errorf("restore coins: %w", err)
	}

	return nil
}

// GetUserHistory returns a user's withdrawal history with transitions.
func (s *Service) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]WithdrawalDetail, bool, error) {
	return s.repo.GetByUser(ctx, userID, limit, offset)
}

// ListAll returns all withdrawals for admin view with filtering.
func (s *Service) ListAll(ctx context.Context, filter ListFilter) ([]WithdrawalListItem, int, error) {
	return s.repo.ListAll(ctx, filter)
}

// UpdateNote allows an admin to edit their own note on a transition.
func (s *Service) UpdateNote(ctx context.Context, adminID string, transitionID uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("note cannot be empty")
	}

	t, err := s.repo.GetTransitionByID(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("get transition: %w", err)
	}
	if t.ActorID != adminID {
		return fmt.Errorf("can only edit your own notes")
	}

	return s.repo.UpdateTransitionNote(ctx, transitionID, note)
}

// GetByID returns a single withdrawal with all transitions.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*WithdrawalDetail, error) {
	return s.repo.GetByID(ctx, id)
}
