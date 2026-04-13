package poll

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeRepo struct {
	poll       *Poll
	rawTallies []VoteTally
	userVote   *int
	insertErr  error
	lastUpdate struct {
		q       string
		opts    []string
		resetOK bool
	}
	updateCalls int
	deleteCount int
}

func (f *fakeRepo) SavePollBatch(ctx context.Context, polls []Poll) error { return nil }
func (f *fakeRepo) GetByContextItemID(ctx context.Context, id uuid.UUID) (*Poll, error) {
	return f.poll, nil
}
func (f *fakeRepo) CountRawTallies(ctx context.Context, pollID uuid.UUID) ([]VoteTally, error) {
	return f.rawTallies, nil
}
func (f *fakeRepo) GetUserVoteIndex(ctx context.Context, userID string, pollID uuid.UUID) (*int, error) {
	return f.userVote, nil
}
func (f *fakeRepo) InsertVote(ctx context.Context, userID string, pollID uuid.UUID, idx int) error {
	return f.insertErr
}
func (f *fakeRepo) UpdatePollAndMaybeResetVotes(ctx context.Context, id uuid.UUID, q string, opts []string, reset bool) error {
	f.updateCalls++
	f.lastUpdate.q, f.lastUpdate.opts, f.lastUpdate.resetOK = q, opts, reset
	return nil
}
func (f *fakeRepo) DeleteByContextItemID(ctx context.Context, id uuid.UUID) error {
	f.deleteCount++
	return nil
}

func newPoll(options ...string) *Poll {
	return &Poll{ID: uuid.New(), ContextItemID: uuid.New(), Question: "Q", Options: options}
}

func TestGetForUser_NoPollReturnsNilNil(t *testing.T) {
	svc := NewService(&fakeRepo{})
	got, err := svc.GetForUser(context.Background(), "u1", uuid.New())
	if err != nil || got != nil {
		t.Fatalf("want nil,nil got %v,%v", got, err)
	}
}

func TestGetForUser_TalliesPaddedWithZeros(t *testing.T) {
	p := newPoll("a", "b", "c")
	svc := NewService(&fakeRepo{poll: p, rawTallies: []VoteTally{{OptionIndex: 1, Count: 4}}})
	got, err := svc.GetForUser(context.Background(), "u1", p.ContextItemID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tallies) != 3 {
		t.Fatalf("want 3 tallies got %d", len(got.Tallies))
	}
	for i, tl := range got.Tallies {
		if tl.OptionIndex != i {
			t.Errorf("tally[%d].OptionIndex=%d", i, tl.OptionIndex)
		}
	}
	if got.Tallies[0].Count != 0 || got.Tallies[1].Count != 4 || got.Tallies[2].Count != 0 {
		t.Errorf("counts=%+v", got.Tallies)
	}
	if got.TotalVotes != 4 {
		t.Errorf("total=%d", got.TotalVotes)
	}
}

func TestGetForUser_AnonymousHasNilUserVote(t *testing.T) {
	p := newPoll("a", "b")
	idx := 1
	svc := NewService(&fakeRepo{poll: p, userVote: &idx})
	got, _ := svc.GetForUser(context.Background(), "", p.ContextItemID)
	if got.UserVoteIndex != nil {
		t.Errorf("anon must have nil UserVoteIndex")
	}
}

func TestGetForUser_LoggedInReceivesUserVote(t *testing.T) {
	p := newPoll("a", "b")
	idx := 1
	svc := NewService(&fakeRepo{poll: p, userVote: &idx})
	got, _ := svc.GetForUser(context.Background(), "u1", p.ContextItemID)
	if got.UserVoteIndex == nil || *got.UserVoteIndex != 1 {
		t.Errorf("want UserVoteIndex=1 got %v", got.UserVoteIndex)
	}
}

func TestVote_RangeChecks(t *testing.T) {
	p := newPoll("a", "b")
	svc := NewService(&fakeRepo{poll: p})
	if err := svc.Vote(context.Background(), "u1", p.ContextItemID, -1); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("-1 want ErrInvalidOption got %v", err)
	}
	if err := svc.Vote(context.Background(), "u1", p.ContextItemID, 2); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("2 want ErrInvalidOption got %v", err)
	}
}

func TestVote_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{poll: nil})
	err := svc.Vote(context.Background(), "u1", uuid.New(), 0)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound got %v", err)
	}
}

func TestVote_DuplicatePropagated(t *testing.T) {
	p := newPoll("a", "b")
	svc := NewService(&fakeRepo{poll: p, insertErr: ErrAlreadyVoted})
	err := svc.Vote(context.Background(), "u1", p.ContextItemID, 0)
	if !errors.Is(err, ErrAlreadyVoted) {
		t.Errorf("want ErrAlreadyVoted got %v", err)
	}
}

func TestVote_Success(t *testing.T) {
	p := newPoll("a", "b")
	svc := NewService(&fakeRepo{poll: p})
	if err := svc.Vote(context.Background(), "u1", p.ContextItemID, 1); err != nil {
		t.Errorf("want nil got %v", err)
	}
}

func TestUpdatePoll_QuestionOnlyNoReset(t *testing.T) {
	p := newPoll("a", "b")
	repo := &fakeRepo{poll: p}
	svc := NewService(repo)
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "new?", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if repo.lastUpdate.resetOK {
		t.Errorf("question-only edit should NOT reset votes")
	}
	if repo.lastUpdate.q != "new?" {
		t.Errorf("question not propagated: %q", repo.lastUpdate.q)
	}
}

func TestUpdatePoll_OptionsChangedTriggersReset(t *testing.T) {
	p := newPoll("a", "b")
	repo := &fakeRepo{poll: p}
	svc := NewService(repo)
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "Q", []string{"a", "c"}); err != nil {
		t.Fatal(err)
	}
	if !repo.lastUpdate.resetOK {
		t.Errorf("options change should reset votes")
	}
}

func TestUpdatePoll_OptionsCountChangedTriggersReset(t *testing.T) {
	p := newPoll("a", "b")
	repo := &fakeRepo{poll: p}
	svc := NewService(repo)
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "Q", []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	if !repo.lastUpdate.resetOK {
		t.Errorf("option count change should reset votes")
	}
}

func TestUpdatePoll_ValidatesOptionCount(t *testing.T) {
	p := newPoll("a", "b")
	svc := NewService(&fakeRepo{poll: p})
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "Q", []string{"a"}); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("1 option want ErrInvalidOption got %v", err)
	}
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "Q", []string{"a", "b", "c", "d", "e"}); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("5 opts want ErrInvalidOption got %v", err)
	}
}

func TestUpdatePoll_RejectsBlankQuestionOrOption(t *testing.T) {
	p := newPoll("a", "b")
	svc := NewService(&fakeRepo{poll: p})
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "   ", []string{"a", "b"}); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("blank question want ErrInvalidOption got %v", err)
	}
	if err := svc.UpdatePoll(context.Background(), p.ContextItemID, "Q", []string{"a", " "}); !errors.Is(err, ErrInvalidOption) {
		t.Errorf("blank option want ErrInvalidOption got %v", err)
	}
}

func TestUpdatePoll_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{poll: nil})
	err := svc.UpdatePoll(context.Background(), uuid.New(), "Q", []string{"a", "b"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound got %v", err)
	}
}

func TestDeletePoll(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.DeletePoll(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if repo.deleteCount != 1 {
		t.Errorf("want 1 delete call got %d", repo.deleteCount)
	}
}
