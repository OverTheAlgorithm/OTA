package terms

import "time"

// Term represents a single terms-of-service record.
// Terms are immutable — once created, they cannot be updated or deleted.
type Term struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Active      bool      `json:"active"`
	Required    bool      `json:"required"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserTermConsent records that a user agreed to a specific term.
type UserTermConsent struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TermID    string    `json:"term_id"`
	CreatedAt time.Time `json:"created_at"`
}
