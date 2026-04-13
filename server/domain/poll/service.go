package poll

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Sentinel errors.
var (
	ErrNotFound      = errors.New("poll not found for article")
	ErrInvalidOption = errors.New("option index out of range or invalid options payload")
	ErrAlreadyVoted  = errors.New("user has already voted on this poll")
)

// Option count bounds for polls.
const (
	MinOptions = 2
	MaxOptions = 4
)

// Service provides business logic for polls.
type Service struct {
	repo Repository
}

// NewService creates a new poll Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetForUser returns the poll with padded tallies and (for logged-in users) the user's vote.
// userID == "" means anonymous — UserVoteIndex is always nil for anonymous viewers.
// Returns (nil, nil) when no poll exists for the article.
func (s *Service) GetForUser(ctx context.Context, userID string, contextItemID uuid.UUID) (*PollForUser, error) {
	p, err := s.repo.GetByContextItemID(ctx, contextItemID)
	if err != nil {
		return nil, fmt.Errorf("get poll: fetch: %w", err)
	}
	if p == nil {
		return nil, nil
	}

	raw, err := s.repo.CountRawTallies(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("get poll: tallies: %w", err)
	}
	tallies := padTallies(raw, len(p.Options))

	total := 0
	for _, t := range tallies {
		total += t.Count
	}

	result := &PollForUser{
		ID:            p.ID,
		ContextItemID: p.ContextItemID,
		Question:      p.Question,
		Options:       p.Options,
		Tallies:       tallies,
		TotalVotes:    total,
	}
	if userID != "" {
		idx, err := s.repo.GetUserVoteIndex(ctx, userID, p.ID)
		if err != nil {
			return nil, fmt.Errorf("get poll: user vote: %w", err)
		}
		result.UserVoteIndex = idx
	}
	return result, nil
}

// Vote records a single user vote. Returns ErrNotFound, ErrInvalidOption, or ErrAlreadyVoted.
func (s *Service) Vote(ctx context.Context, userID string, contextItemID uuid.UUID, optionIndex int) error {
	p, err := s.repo.GetByContextItemID(ctx, contextItemID)
	if err != nil {
		return fmt.Errorf("vote: fetch: %w", err)
	}
	if p == nil {
		return ErrNotFound
	}
	if optionIndex < 0 || optionIndex >= len(p.Options) {
		return ErrInvalidOption
	}
	if err := s.repo.InsertVote(ctx, userID, p.ID, optionIndex); err != nil {
		if errors.Is(err, ErrAlreadyVoted) {
			return ErrAlreadyVoted
		}
		return fmt.Errorf("vote: insert: %w", err)
	}
	return nil
}

// UpdatePoll applies admin edits. Votes are reset only when options content/order changes;
// edits to question text alone preserve votes.
func (s *Service) UpdatePoll(ctx context.Context, contextItemID uuid.UUID, question string, options []string) error {
	if err := validatePollPayload(question, options); err != nil {
		return err
	}
	p, err := s.repo.GetByContextItemID(ctx, contextItemID)
	if err != nil {
		return fmt.Errorf("update poll: fetch: %w", err)
	}
	if p == nil {
		return ErrNotFound
	}
	reset := !sameOptions(p.Options, options)
	if err := s.repo.UpdatePollAndMaybeResetVotes(ctx, p.ID, question, options, reset); err != nil {
		return fmt.Errorf("update poll: persist: %w", err)
	}
	return nil
}

// DeletePoll removes a poll (and its votes via ON DELETE CASCADE).
func (s *Service) DeletePoll(ctx context.Context, contextItemID uuid.UUID) error {
	if err := s.repo.DeleteByContextItemID(ctx, contextItemID); err != nil {
		return fmt.Errorf("delete poll: %w", err)
	}
	return nil
}

func validatePollPayload(question string, options []string) error {
	if strings.TrimSpace(question) == "" {
		return ErrInvalidOption
	}
	if len(options) < MinOptions || len(options) > MaxOptions {
		return ErrInvalidOption
	}
	for _, o := range options {
		if strings.TrimSpace(o) == "" {
			return ErrInvalidOption
		}
	}
	return nil
}

func sameOptions(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func padTallies(raw []VoteTally, n int) []VoteTally {
	byIdx := make(map[int]int, len(raw))
	for _, t := range raw {
		byIdx[t.OptionIndex] = t.Count
	}
	out := make([]VoteTally, n)
	for i := 0; i < n; i++ {
		out[i] = VoteTally{OptionIndex: i, Count: byIdx[i]}
	}
	return out
}
