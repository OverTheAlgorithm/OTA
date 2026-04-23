package withdrawal

import (
	"context"
	"errors"
	"testing"

	"ota/domain/apperr"

	"github.com/google/uuid"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockRepoForService struct {
	bankAccount    *BankAccount
	bankAccountErr error
	createID       uuid.UUID
	createErr      error
}

func (m *mockRepoForService) GetBankAccount(_ context.Context, _ string) (*BankAccount, error) {
	return m.bankAccount, m.bankAccountErr
}
func (m *mockRepoForService) UpsertBankAccount(_ context.Context, _ BankAccount) error { return nil }
func (m *mockRepoForService) CreateWithdrawalWithDeduction(_ context.Context, _ Withdrawal, _ string, _ int) (uuid.UUID, error) {
	return m.createID, m.createErr
}
func (m *mockRepoForService) CreateWithdrawal(_ context.Context, _ Withdrawal, _ string) (uuid.UUID, error) {
	return m.createID, m.createErr
}
func (m *mockRepoForService) GetByID(_ context.Context, _ uuid.UUID) (*WithdrawalDetail, error) {
	return nil, nil
}
func (m *mockRepoForService) GetByUser(_ context.Context, _ string, _, _ int) ([]WithdrawalDetail, bool, error) {
	return nil, false, nil
}
func (m *mockRepoForService) ListAll(_ context.Context, _ ListFilter) ([]WithdrawalListItem, int, error) {
	return nil, 0, nil
}
func (m *mockRepoForService) AddTransition(_ context.Context, _ uuid.UUID, _, _, _ string) error {
	return nil
}
func (m *mockRepoForService) GetLatestStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockRepoForService) GetTransitionByID(_ context.Context, _ uuid.UUID) (*Transition, error) {
	return nil, nil
}
func (m *mockRepoForService) UpdateTransitionNote(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockRepoForService) CancelWithdrawalAtomic(_ context.Context, _ uuid.UUID, _ string) (int, string, error) {
	return 0, "", nil
}
func (m *mockRepoForService) RejectWithdrawalAtomic(_ context.Context, _ uuid.UUID, _, _ string) (int, string, error) {
	return 0, "", nil
}
func (m *mockRepoForService) ApproveWithdrawalAtomic(_ context.Context, _ uuid.UUID, _, _ string) error {
	return nil
}
func (m *mockRepoForService) GetWithdrawalOwner(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockRepoForService) HasPendingWithdrawals(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type mockCoinManagerForService struct {
	coins    int
	coinsErr error
}

func (m *mockCoinManagerForService) GetUserCoins(_ context.Context, _ string) (int, error) {
	return m.coins, m.coinsErr
}
func (m *mockCoinManagerForService) DeductCoins(_ context.Context, _ string, _ int) error { return nil }
func (m *mockCoinManagerForService) RestoreCoins(_ context.Context, _ string, _ int) error {
	return nil
}

// ── GetPreCheckInfo tests ─────────────────────────────────────────────────────

func TestGetPreCheckInfo_WithBankAccount(t *testing.T) {
	repo := &mockRepoForService{
		bankAccount: &BankAccount{
			UserID:        "user1",
			BankName:      "KB",
			AccountNumber: "1234",
			AccountHolder: "홍길동",
		},
	}
	coinMgr := &mockCoinManagerForService{coins: 3000}
	svc := NewService(repo, coinMgr, 1000, 1000)

	info, err := svc.GetPreCheckInfo(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.CurrentBalance != 3000 {
		t.Errorf("expected balance 3000, got %d", info.CurrentBalance)
	}
	if !info.HasBankAccount {
		t.Error("expected HasBankAccount true")
	}
	if info.MinWithdrawalAmount != 1000 {
		t.Errorf("expected min 1000, got %d", info.MinWithdrawalAmount)
	}
	if info.WithdrawalUnitAmount != 1000 {
		t.Errorf("expected unit 1000, got %d", info.WithdrawalUnitAmount)
	}
}

func TestGetPreCheckInfo_NoBankAccount(t *testing.T) {
	repo := &mockRepoForService{bankAccount: nil}
	coinMgr := &mockCoinManagerForService{coins: 500}
	svc := NewService(repo, coinMgr, 1000, 1000)

	info, err := svc.GetPreCheckInfo(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.HasBankAccount {
		t.Error("expected HasBankAccount false")
	}
	if info.CurrentBalance != 500 {
		t.Errorf("expected balance 500, got %d", info.CurrentBalance)
	}
}

func TestGetPreCheckInfo_CoinError(t *testing.T) {
	repo := &mockRepoForService{}
	coinMgr := &mockCoinManagerForService{coinsErr: errors.New("db error")}
	svc := NewService(repo, coinMgr, 1000, 1000)

	_, err := svc.GetPreCheckInfo(context.Background(), "user1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetPreCheckInfo_BankAccountError(t *testing.T) {
	repo := &mockRepoForService{bankAccountErr: errors.New("db error")}
	coinMgr := &mockCoinManagerForService{coins: 1000}
	svc := NewService(repo, coinMgr, 1000, 1000)

	_, err := svc.GetPreCheckInfo(context.Background(), "user1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── normalizeBankName tests ──────────────────────────────────────────────────

func TestNormalizeBankName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"신한은행", "신한"},
		{"신한 은행", "신한"},
		{"   신한  은행", "신한"},
		{"카카오뱅크", "카카오뱅크"},
		{"KB 국민 은행", "KB국민"},
		{"  하나  ", "하나"},
		{"우리은행", "우리"},
		{"은행", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeBankName(tt.input)
			if got != tt.want {
				t.Errorf("normalizeBankName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── RequestWithdrawal unit amount validation tests ───────────────────────────

func TestRequestWithdrawal_UnitAmountValidation(t *testing.T) {
	bankAccount := &BankAccount{
		UserID:        "user1",
		BankName:      "KB",
		AccountNumber: "1234",
		AccountHolder: "홍길동",
	}

	tests := []struct {
		name       string
		amount     int
		unitAmount int
		wantErr    bool
		errType    string
	}{
		{
			name:       "valid multiple",
			amount:     2000,
			unitAmount: 1000,
			wantErr:    false,
		},
		{
			name:       "not a multiple",
			amount:     1500,
			unitAmount: 1000,
			wantErr:    true,
			errType:    "validation",
		},
		{
			name:       "exact minimum multiple",
			amount:     1000,
			unitAmount: 1000,
			wantErr:    false,
		},
		{
			name:       "unit 500, valid amount",
			amount:     1500,
			unitAmount: 500,
			wantErr:    false,
		},
		{
			name:       "unit 500, invalid amount",
			amount:     1250,
			unitAmount: 500,
			wantErr:    true,
			errType:    "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepoForService{
				bankAccount: bankAccount,
				createID:    uuid.New(),
			}
			coinMgr := &mockCoinManagerForService{coins: 5000}
			// min = tt.unitAmount so it's always a multiple of unit
			svc := NewService(repo, coinMgr, tt.unitAmount, tt.unitAmount)

			_, err := svc.RequestWithdrawal(context.Background(), "user1", tt.amount)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errType == "validation" {
					var ve *apperr.ValidationError
					if !errors.As(err, &ve) {
						t.Errorf("expected ValidationError, got %T: %v", err, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
