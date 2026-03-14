package terms

import (
	"context"
	"fmt"
)

// Service encapsulates terms business logic.
type Service struct {
	repo Repository
}

// NewService creates a new terms service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateTerm inserts a new immutable term.
func (s *Service) CreateTerm(ctx context.Context, t Term) (Term, error) {
	if t.Title == "" {
		return Term{}, fmt.Errorf("title is required")
	}
	if t.URL == "" {
		return Term{}, fmt.Errorf("url is required")
	}
	if t.Version == "" {
		return Term{}, fmt.Errorf("version is required")
	}
	return s.repo.Create(ctx, t)
}

// ListAllTerms returns all terms (admin).
func (s *Service) ListAllTerms(ctx context.Context) ([]Term, error) {
	return s.repo.ListAll(ctx)
}

// GetActiveTerms returns active terms for the consent screen.
func (s *Service) GetActiveTerms(ctx context.Context) ([]Term, error) {
	return s.repo.ListActive(ctx)
}

// ValidateConsents checks that all active+required terms are present in agreedTermIDs.
// Returns nil if valid, error if any required term is missing.
func (s *Service) ValidateConsents(ctx context.Context, agreedTermIDs []string) error {
	requiredTerms, err := s.repo.FindActiveRequired(ctx)
	if err != nil {
		return fmt.Errorf("loading required terms: %w", err)
	}

	agreedSet := make(map[string]struct{}, len(agreedTermIDs))
	for _, id := range agreedTermIDs {
		agreedSet[id] = struct{}{}
	}

	for _, t := range requiredTerms {
		if _, ok := agreedSet[t.ID]; !ok {
			return fmt.Errorf("필수 약관에 동의하지 않았습니다: %s (v%s)", t.Title, t.Version)
		}
	}

	return nil
}

// UpdateTermActive toggles the active status of a term.
func (s *Service) UpdateTermActive(ctx context.Context, termID string, active bool) error {
	if termID == "" {
		return fmt.Errorf("term ID is required")
	}
	return s.repo.UpdateActive(ctx, termID, active)
}

// UpdateTerm modifies mutable fields (url, description, required) of an existing term.
func (s *Service) UpdateTerm(ctx context.Context, termID, url, description string, required bool) (Term, error) {
	if termID == "" {
		return Term{}, fmt.Errorf("term ID is required")
	}
	if url == "" {
		return Term{}, fmt.Errorf("url is required")
	}
	return s.repo.Update(ctx, termID, url, description, required)
}

// SaveConsents persists consent records after validation passes.
func (s *Service) SaveConsents(ctx context.Context, userID string, termIDs []string) error {
	return s.repo.SaveConsents(ctx, userID, termIDs)
}
