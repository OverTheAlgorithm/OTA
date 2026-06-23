package communitytrend

import (
	"context"
	"fmt"
	"time"
)

// Worksheet is the per-(community, date) unit of tagging work. It stores status
// only — never the posts or titles (copyright guardrail).
type Worksheet struct {
	ID          int        `json:"id"`
	CommunityID int        `json:"community_id"`
	StatDate    time.Time  `json:"stat_date"`
	Mode        string     `json:"mode"`   // 'auto' | 'manual'
	Status      string     `json:"status"` // 'pending' | 'suggested' | 'confirmed'
	TotalPosts  *int       `json:"total_posts"`
	ConfirmedBy *string    `json:"confirmed_by"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
}

// TagCount is one confirmed (tag, count) pair for a community-day.
type TagCount struct {
	TagID int `json:"tag_id"`
	Count int `json:"count"`
}

// Confirmation is the atomic payload written when a worksheet is confirmed.
// Both the auto path (adapter+AI) and the manual path produce this shape;
// they merge in ct_tag_daily keyed by community (decisions.md D-004).
type Confirmation struct {
	CommunityID  int
	StatDate     time.Time
	Mode         string // 'auto' | 'manual'
	Counts       []TagCount
	TotalPosts   int
	Source       string  // 'ai' | 'human' | 'hybrid'
	ConfirmedBy  *string // user id, nullable
	Fingerprints []string
}

// WorksheetRepository persists worksheets and the atomic confirmation write.
type WorksheetRepository interface {
	// Ensure creates a pending worksheet for (community, date, mode) if absent and returns it.
	Ensure(ctx context.Context, communityID int, date time.Time, mode string) (Worksheet, error)
	ListByDate(ctx context.Context, date time.Time) ([]Worksheet, error)
	// Confirm atomically writes tag_daily + community_daily + seen fingerprints
	// and marks the worksheet confirmed.
	Confirm(ctx context.Context, conf Confirmation) error
}

// WorksheetService validates and orchestrates worksheet confirmation.
type WorksheetService struct {
	worksheets WorksheetRepository
}

func NewWorksheetService(worksheets WorksheetRepository) *WorksheetService {
	return &WorksheetService{worksheets: worksheets}
}

func (s *WorksheetService) ListByDate(ctx context.Context, date time.Time) ([]Worksheet, error) {
	return s.worksheets.ListByDate(ctx, date)
}

func (s *WorksheetService) Ensure(ctx context.Context, communityID int, date time.Time, mode string) (Worksheet, error) {
	if mode != "auto" && mode != "manual" {
		return Worksheet{}, fmt.Errorf("mode는 auto 또는 manual이어야 합니다")
	}
	return s.worksheets.Ensure(ctx, communityID, date, mode)
}

// Confirm validates a confirmation and writes it atomically.
func (s *WorksheetService) Confirm(ctx context.Context, conf Confirmation) error {
	if conf.CommunityID <= 0 {
		return fmt.Errorf("커뮤니티 ID가 필요합니다")
	}
	if conf.Mode != "auto" && conf.Mode != "manual" {
		return fmt.Errorf("mode는 auto 또는 manual이어야 합니다")
	}
	if conf.Source != "ai" && conf.Source != "human" && conf.Source != "hybrid" {
		return fmt.Errorf("source는 ai/human/hybrid 중 하나여야 합니다")
	}
	if conf.TotalPosts < 0 {
		return fmt.Errorf("총 글 수는 음수일 수 없습니다")
	}
	for _, c := range conf.Counts {
		if c.TagID <= 0 {
			return fmt.Errorf("태그 ID가 올바르지 않습니다")
		}
		if c.Count < 0 {
			return fmt.Errorf("카운트는 음수일 수 없습니다")
		}
	}
	return s.worksheets.Confirm(ctx, conf)
}
