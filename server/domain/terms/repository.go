package terms

import "context"

// Repository defines data access for terms and user consents.
type Repository interface {
	// Create inserts a new immutable term record.
	Create(ctx context.Context, t Term) (Term, error)

	// ListAll returns all terms regardless of active status (admin view).
	ListAll(ctx context.Context) ([]Term, error)

	// ListActive returns only active terms (user-facing consent screen).
	ListActive(ctx context.Context) ([]Term, error)

	// FindActiveRequired returns active terms where required=true (for server-side validation).
	FindActiveRequired(ctx context.Context) ([]Term, error)

	// SaveConsents batch-inserts consent records for a user.
	SaveConsents(ctx context.Context, userID string, termIDs []string) error

	// UpdateActive toggles the active status of a term.
	UpdateActive(ctx context.Context, termID string, active bool) error

	// GetUserConsents returns all consent records for a user.
	GetUserConsents(ctx context.Context, userID string) ([]UserTermConsent, error)
}
