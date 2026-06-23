package communitytrend

import (
	"context"
	"fmt"
	"time"
)

// Meme is a confirmed meme. Status 'retired' replaces hard delete so historical
// ct_meme_daily counts survive (decisions.md D-011).
type Meme struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Aliases    []string  `json:"aliases"`
	Status     string    `json:"status"`      // 'active' | 'retired'
	CreatedVia string    `json:"created_via"` // 'promote' | 'manual'
	CreatedAt  time.Time `json:"created_at"`
}

// CandidateRow is a stored meme candidate awaiting human promotion/rejection.
// Only the AI creates candidates; humans promote or reject (decisions.md D-012).
type CandidateRow struct {
	ID         int       `json:"id"`
	Expression string    `json:"expression"`
	HitCount   int       `json:"hit_count"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

// MemeRepository persists the meme track (confirmed memes, candidates, blacklist).
type MemeRepository interface {
	// candidates (AI-driven create; human promote/reject)
	UpsertCandidate(ctx context.Context, expression string, date time.Time) error // no-op if blacklisted; else insert or hit_count++
	ListCandidates(ctx context.Context) ([]CandidateRow, error)
	RejectCandidate(ctx context.Context, id int) error // delete + blacklist its expression (permanent)
	PromoteCandidate(ctx context.Context, id int, name string, aliases []string) (Meme, error)

	// confirmed memes
	CreateMeme(ctx context.Context, name string, aliases []string) (Meme, error) // created_via='manual'
	ListMemes(ctx context.Context, includeRetired bool) ([]Meme, error)
	UpdateMeme(ctx context.Context, id int, name string, aliases []string) (Meme, error)
	RetireMeme(ctx context.Context, id int) error
}

// MemeService validates and orchestrates the meme state machine.
type MemeService struct {
	repo MemeRepository
}

func NewMemeService(repo MemeRepository) *MemeService { return &MemeService{repo: repo} }

func (s *MemeService) ListCandidates(ctx context.Context) ([]CandidateRow, error) {
	return s.repo.ListCandidates(ctx)
}

func (s *MemeService) RejectCandidate(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("후보 ID가 올바르지 않습니다")
	}
	return s.repo.RejectCandidate(ctx, id)
}

func (s *MemeService) PromoteCandidate(ctx context.Context, id int, name string, aliases []string) (Meme, error) {
	if id <= 0 {
		return Meme{}, fmt.Errorf("후보 ID가 올바르지 않습니다")
	}
	if name == "" {
		return Meme{}, fmt.Errorf("밈 이름은 필수입니다")
	}
	return s.repo.PromoteCandidate(ctx, id, name, aliases)
}

func (s *MemeService) CreateMeme(ctx context.Context, name string, aliases []string) (Meme, error) {
	if name == "" {
		return Meme{}, fmt.Errorf("밈 이름은 필수입니다")
	}
	return s.repo.CreateMeme(ctx, name, aliases)
}

func (s *MemeService) ListMemes(ctx context.Context, includeRetired bool) ([]Meme, error) {
	return s.repo.ListMemes(ctx, includeRetired)
}

func (s *MemeService) UpdateMeme(ctx context.Context, id int, name string, aliases []string) (Meme, error) {
	if id <= 0 {
		return Meme{}, fmt.Errorf("밈 ID가 올바르지 않습니다")
	}
	if name == "" {
		return Meme{}, fmt.Errorf("밈 이름은 필수입니다")
	}
	return s.repo.UpdateMeme(ctx, id, name, aliases)
}

func (s *MemeService) RetireMeme(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("밈 ID가 올바르지 않습니다")
	}
	return s.repo.RetireMeme(ctx, id)
}
