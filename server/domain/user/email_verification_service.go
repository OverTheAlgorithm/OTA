package user

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/google/uuid"
)

const (
	CodeLength         = 6
	CodeExpiryDuration = 5 * time.Minute
	MaxCodesPerHour    = 5
	RateLimitWindow    = 1 * time.Hour
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type EmailVerificationService struct {
	verificationRepo EmailVerificationRepository
	userRepo         Repository
}

func NewEmailVerificationService(
	verificationRepo EmailVerificationRepository,
	userRepo Repository,
) *EmailVerificationService {
	return &EmailVerificationService{
		verificationRepo: verificationRepo,
		userRepo:         userRepo,
	}
}

type SendCodeResult struct {
	Code  string // The generated code (needed by handler to send email)
	Email string
}

// SendCode validates email, checks rate limit, creates code
func (s *EmailVerificationService) SendCode(ctx context.Context, userID string, email string) (SendCodeResult, error) {
	// 1. Validate email format
	if !emailRegex.MatchString(email) {
		return SendCodeResult{}, fmt.Errorf("invalid email format")
	}

	// 2. Check rate limit
	since := time.Now().UTC().Add(-RateLimitWindow)
	count, err := s.verificationRepo.CountRecentCodes(ctx, userID, since)
	if err != nil {
		return SendCodeResult{}, fmt.Errorf("rate limit check failed: %w", err)
	}
	if count >= MaxCodesPerHour {
		return SendCodeResult{}, fmt.Errorf("rate limit exceeded: max %d codes per hour", MaxCodesPerHour)
	}

	// 3. Generate 6-digit code
	code, err := generateCode(CodeLength)
	if err != nil {
		return SendCodeResult{}, fmt.Errorf("code generation failed: %w", err)
	}

	// 4. Store code
	verificationCode := EmailVerificationCode{
		ID:        uuid.New().String(),
		UserID:    userID,
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().UTC().Add(CodeExpiryDuration),
		Used:      false,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.verificationRepo.CreateCode(ctx, verificationCode); err != nil {
		return SendCodeResult{}, fmt.Errorf("failed to store code: %w", err)
	}

	return SendCodeResult{Code: code, Email: email}, nil
}

// VerifyCode validates the code and updates the user's email
func (s *EmailVerificationService) VerifyCode(ctx context.Context, userID string, code string) error {
	// 1. Find valid (unexpired, unused) code
	verificationCode, err := s.verificationRepo.FindValidCode(ctx, userID, code)
	if err != nil {
		return fmt.Errorf("invalid or expired verification code")
	}

	// 2. Check expiry (defense in depth; query also filters)
	if time.Now().UTC().After(verificationCode.ExpiresAt) {
		return fmt.Errorf("verification code has expired")
	}

	// 3. Mark code as used
	if err := s.verificationRepo.MarkCodeUsed(ctx, verificationCode.ID); err != nil {
		return fmt.Errorf("failed to mark code as used: %w", err)
	}

	// 4. Update user email
	if err := s.userRepo.UpdateEmail(ctx, userID, verificationCode.Email); err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	// 5. Invalidate other pending codes for this user
	_ = s.verificationRepo.InvalidatePendingCodes(ctx, userID)

	return nil
}

// generateCode produces a cryptographically random N-digit numeric code
func generateCode(length int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n), nil
}
