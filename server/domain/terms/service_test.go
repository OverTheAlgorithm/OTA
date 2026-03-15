package terms

import (
	"context"
	"fmt"
	"testing"
)

// ─── Mock Repository ────────────────────────────────────────────────────────

type mockRepo struct {
	terms           []Term
	activeTerms     []Term
	requiredTerms   []Term
	savedConsents   map[string][]string // userID -> termIDs
	createErr       error
	listAllErr      error
	listActiveErr   error
	findRequiredErr error
	saveConsentsErr error
	updateActiveErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{savedConsents: make(map[string][]string)}
}

func (m *mockRepo) Create(_ context.Context, t Term) (Term, error) {
	if m.createErr != nil {
		return Term{}, m.createErr
	}
	t.ID = "generated-id"
	m.terms = append(m.terms, t)
	return t, nil
}

func (m *mockRepo) ListAll(_ context.Context) ([]Term, error) {
	if m.listAllErr != nil {
		return nil, m.listAllErr
	}
	return m.terms, nil
}

func (m *mockRepo) ListActive(_ context.Context) ([]Term, error) {
	if m.listActiveErr != nil {
		return nil, m.listActiveErr
	}
	return m.activeTerms, nil
}

func (m *mockRepo) FindActiveRequired(_ context.Context) ([]Term, error) {
	if m.findRequiredErr != nil {
		return nil, m.findRequiredErr
	}
	return m.requiredTerms, nil
}

func (m *mockRepo) SaveConsents(_ context.Context, userID string, termIDs []string) error {
	if m.saveConsentsErr != nil {
		return m.saveConsentsErr
	}
	m.savedConsents[userID] = termIDs
	return nil
}

func (m *mockRepo) UpdateActive(_ context.Context, termID string, active bool) error {
	if m.updateActiveErr != nil {
		return m.updateActiveErr
	}
	for i, t := range m.terms {
		if t.ID == termID {
			m.terms[i].Active = active
			return nil
		}
	}
	return fmt.Errorf("term not found")
}

func (m *mockRepo) Update(_ context.Context, termID string, url, description string, required bool) (Term, error) {
	for i, t := range m.terms {
		if t.ID == termID {
			m.terms[i].URL = url
			m.terms[i].Description = description
			m.terms[i].Required = required
			return m.terms[i], nil
		}
	}
	return Term{}, fmt.Errorf("term not found")
}

func (m *mockRepo) GetUserConsents(_ context.Context, userID string) ([]UserTermConsent, error) {
	return nil, nil
}

// ─── Service Tests ──────────────────────────────────────────────────────────

func TestCreateTerm_Validation(t *testing.T) {
	tests := []struct {
		name    string
		term    Term
		wantErr string
	}{
		{
			name:    "empty title",
			term:    Term{URL: "https://example.com", Version: "1"},
			wantErr: "title is required",
		},
		{
			name:    "empty url",
			term:    Term{Title: "Terms", Version: "1"},
			wantErr: "url is required",
		},
		{
			name:    "empty version",
			term:    Term{Title: "Terms", URL: "https://example.com"},
			wantErr: "version is required",
		},
		{
			name: "valid term",
			term: Term{Title: "Terms", URL: "https://example.com", Version: "1", Active: true, Required: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			svc := NewService(repo)

			_, err := svc.CreateTerm(context.Background(), tt.term)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCreateTerm_Success(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	created, err := svc.CreateTerm(context.Background(), Term{
		Title:    "개인정보 처리방침",
		URL:      "https://notion.so/privacy",
		Version:  "1.2",
		Active:   true,
		Required: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.Title != "개인정보 처리방침" {
		t.Fatalf("expected title '개인정보 처리방침', got %q", created.Title)
	}
	if len(repo.terms) != 1 {
		t.Fatalf("expected 1 term in repo, got %d", len(repo.terms))
	}
}

func TestCreateTerm_RepoError(t *testing.T) {
	repo := newMockRepo()
	repo.createErr = fmt.Errorf("duplicate key")
	svc := NewService(repo)

	_, err := svc.CreateTerm(context.Background(), Term{
		Title: "Terms", URL: "https://example.com", Version: "1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateConsents_AllRequiredAgreed(t *testing.T) {
	repo := newMockRepo()
	repo.requiredTerms = []Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
		{ID: "term-2", Title: "TOS", Version: "1", Required: true},
	}
	svc := NewService(repo)

	err := svc.ValidateConsents(context.Background(), []string{"term-1", "term-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConsents_MissingRequired(t *testing.T) {
	repo := newMockRepo()
	repo.requiredTerms = []Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
		{ID: "term-2", Title: "TOS", Version: "1", Required: true},
	}
	svc := NewService(repo)

	err := svc.ValidateConsents(context.Background(), []string{"term-1"})
	if err == nil {
		t.Fatal("expected error for missing required term, got nil")
	}
}

func TestValidateConsents_EmptyAgreedList(t *testing.T) {
	repo := newMockRepo()
	repo.requiredTerms = []Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
	}
	svc := NewService(repo)

	err := svc.ValidateConsents(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateConsents_NoRequiredTerms(t *testing.T) {
	repo := newMockRepo()
	repo.requiredTerms = []Term{}
	svc := NewService(repo)

	// No required terms — any agreement (even empty) should pass
	err := svc.ValidateConsents(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConsents_ExtraOptionalTermsOK(t *testing.T) {
	repo := newMockRepo()
	repo.requiredTerms = []Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
	}
	svc := NewService(repo)

	// User agreed to required + optional
	err := svc.ValidateConsents(context.Background(), []string{"term-1", "optional-term-99"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConsents_RepoError(t *testing.T) {
	repo := newMockRepo()
	repo.findRequiredErr = fmt.Errorf("db error")
	svc := NewService(repo)

	err := svc.ValidateConsents(context.Background(), []string{"term-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSaveConsents_Success(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	err := svc.SaveConsents(context.Background(), "user-1", []string{"term-1", "term-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved, ok := repo.savedConsents["user-1"]
	if !ok {
		t.Fatal("expected consents saved for user-1")
	}
	if len(saved) != 2 {
		t.Fatalf("expected 2 consents, got %d", len(saved))
	}
}

func TestSaveConsents_RepoError(t *testing.T) {
	repo := newMockRepo()
	repo.saveConsentsErr = fmt.Errorf("constraint violation")
	svc := NewService(repo)

	err := svc.SaveConsents(context.Background(), "user-1", []string{"term-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListTerms(t *testing.T) {
	repo := newMockRepo()
	repo.terms = []Term{
		{ID: "1", Title: "A", Active: true},
		{ID: "2", Title: "B", Active: false},
	}
	repo.activeTerms = []Term{repo.terms[0]}
	svc := NewService(repo)

	all, err := svc.ListAllTerms(context.Background())
	if err != nil {
		t.Fatalf("ListAllTerms: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListAllTerms = %d, want 2", len(all))
	}

	active, err := svc.GetActiveTerms(context.Background())
	if err != nil {
		t.Fatalf("GetActiveTerms: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("GetActiveTerms = %d, want 1", len(active))
	}
}

func TestUpdateTermActive(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		terms   []Term
		wantErr bool
	}{
		{"success", "t-1", []Term{{ID: "t-1", Title: "Privacy", Active: true}}, false},
		{"empty ID", "", nil, true},
		{"not found", "nonexistent", nil, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			repo.terms = tt.terms
			err := NewService(repo).UpdateTermActive(context.Background(), tt.id, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
