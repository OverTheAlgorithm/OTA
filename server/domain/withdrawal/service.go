package withdrawal

import (
	"context"
	"fmt"
	"strings"

	"ota/domain/apperr"

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
	repo                 Repository
	coinManager          CoinManager
	minWithdrawalAmount  int
	withdrawalUnitAmount int
}

func NewService(repo Repository, coinManager CoinManager, minWithdrawalAmount, withdrawalUnitAmount int) *Service {
	return &Service{
		repo:                 repo,
		coinManager:          coinManager,
		minWithdrawalAmount:  minWithdrawalAmount,
		withdrawalUnitAmount: withdrawalUnitAmount,
	}
}

// GetMinWithdrawalAmount returns the configured minimum withdrawal amount.
func (s *Service) GetMinWithdrawalAmount() int {
	return s.minWithdrawalAmount
}

// GetWithdrawalUnitAmount returns the configured withdrawal unit amount.
func (s *Service) GetWithdrawalUnitAmount() int {
	return s.withdrawalUnitAmount
}

// GetPreCheckInfo returns data needed by the frontend before showing the withdrawal modal.
func (s *Service) GetPreCheckInfo(ctx context.Context, userID string) (PreCheckInfo, error) {
	coins, err := s.coinManager.GetUserCoins(ctx, userID)
	if err != nil {
		return PreCheckInfo{}, fmt.Errorf("get user coins: %w", err)
	}

	account, err := s.repo.GetBankAccount(ctx, userID)
	if err != nil {
		return PreCheckInfo{}, fmt.Errorf("get bank account: %w", err)
	}

	return PreCheckInfo{
		MinWithdrawalAmount:  s.minWithdrawalAmount,
		WithdrawalUnitAmount: s.withdrawalUnitAmount,
		CurrentBalance:       coins,
		HasBankAccount:       account != nil,
	}, nil
}

// GetBankAccount returns the user's registered bank account, or nil if none.
func (s *Service) GetBankAccount(ctx context.Context, userID string) (*BankAccount, error) {
	return s.repo.GetBankAccount(ctx, userID)
}

// normalizeBankName removes all whitespace, strips trailing "은행" suffix, and trims.
// e.g. "신한 은행" → "신한", "   신한  은행" → "신한", "카카오뱅크" → "카카오뱅크"
func normalizeBankName(name string) string {
	// Remove all whitespace characters
	var b strings.Builder
	for _, r := range name {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			b.WriteRune(r)
		}
	}
	n := b.String()
	// Strip "은행" suffix if present
	n = strings.TrimSuffix(n, "은행")
	return strings.TrimSpace(n)
}

// SaveBankAccount creates or updates the user's bank account info.
func (s *Service) SaveBankAccount(ctx context.Context, account BankAccount) error {
	account.BankName = normalizeBankName(account.BankName)
	if account.BankName == "" {
		return apperr.NewValidationError("bank_name", "is required")
	}
	if strings.TrimSpace(account.AccountNumber) == "" {
		return apperr.NewValidationError("account_number", "is required")
	}
	if strings.TrimSpace(account.AccountHolder) == "" {
		return apperr.NewValidationError("account_holder", "is required")
	}
	return s.repo.UpsertBankAccount(ctx, account)
}

// RequestWithdrawal creates a new withdrawal request.
// Validates: bank account registered, minimum amount.
// Uses an atomic DB transaction (SELECT FOR UPDATE) to prevent double-spend
// race conditions — balance check, deduction, and withdrawal creation all
// happen in a single transaction.
func (s *Service) RequestWithdrawal(ctx context.Context, userID string, amount int) (*Withdrawal, error) {
	// 1. Check bank account
	account, err := s.repo.GetBankAccount(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check bank account: %w", err)
	}
	if account == nil {
		return nil, apperr.ErrBankAccountRequired
	}

	// 2. Validate minimum amount
	if amount < s.minWithdrawalAmount {
		return nil, apperr.NewMinimumAmountError(s.minWithdrawalAmount)
	}

	// 2b. Validate unit amount
	if amount%s.withdrawalUnitAmount != 0 {
		return nil, apperr.NewValidationError("amount", fmt.Sprintf("must be a multiple of %d", s.withdrawalUnitAmount))
	}

	// 3. Atomic: lock balance row, check, deduct, create withdrawal + transition
	w := Withdrawal{
		UserID:        userID,
		Amount:        amount,
		BankName:      account.BankName,
		AccountNumber: account.AccountNumber,
		AccountHolder: account.AccountHolder,
	}
	id, err := s.repo.CreateWithdrawalWithDeduction(ctx, w, userID, amount)
	if err != nil {
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	w.ID = id
	w.CurrentStatus = StatusPending
	return &w, nil
}

// CancelWithdrawal cancels a pending withdrawal and restores coins atomically.
func (s *Service) CancelWithdrawal(ctx context.Context, userID string, withdrawalID uuid.UUID) error {
	// 1. Verify ownership before entering the transaction
	owner, err := s.repo.GetWithdrawalOwner(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get withdrawal owner: %w", err)
	}
	if owner != userID {
		return apperr.ErrUnauthorized
	}

	// 2. Atomically: verify pending, insert transition, restore coins
	if _, _, err := s.repo.CancelWithdrawalAtomic(ctx, withdrawalID, userID); err != nil {
		return fmt.Errorf("cancel withdrawal: %w", err)
	}

	return nil
}

// ApproveWithdrawal approves a pending withdrawal (admin action).
func (s *Service) ApproveWithdrawal(ctx context.Context, adminID string, withdrawalID uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return apperr.NewValidationError("note", "is required")
	}

	status, err := s.repo.GetLatestStatus(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if status != StatusPending {
		return apperr.NewConflictError(fmt.Sprintf("can only approve pending withdrawals (current: %s)", status))
	}

	return s.repo.AddTransition(ctx, withdrawalID, StatusApproved, note, adminID)
}

// RejectWithdrawal rejects a pending withdrawal (admin action) and restores coins atomically.
func (s *Service) RejectWithdrawal(ctx context.Context, adminID string, withdrawalID uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return apperr.NewValidationError("note", "rejection reason is required")
	}

	// Atomically: verify pending, insert transition with note, restore coins
	if _, _, err := s.repo.RejectWithdrawalAtomic(ctx, withdrawalID, adminID, note); err != nil {
		return fmt.Errorf("reject withdrawal: %w", err)
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
		return apperr.NewValidationError("note", "cannot be empty")
	}

	t, err := s.repo.GetTransitionByID(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("get transition: %w", err)
	}
	if t.ActorID != adminID {
		return apperr.ErrUnauthorized
	}

	return s.repo.UpdateTransitionNote(ctx, transitionID, note)
}

// GetByID returns a single withdrawal with all transitions.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*WithdrawalDetail, error) {
	return s.repo.GetByID(ctx, id)
}
