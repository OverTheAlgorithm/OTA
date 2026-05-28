package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/cache"
	"ota/domain/comment"
)

// fakeRepo captures ApplyCounters and UpsertReactions calls.
type fakeRepo struct {
	mu             sync.Mutex
	counterCalls   []counterCall
	reactionCalls  []reactionCall
	upsertErr      error
	applyErr       error
}

type counterCall struct {
	id            uuid.UUID
	likes, dislikes int
}

type reactionCall struct {
	id   uuid.UUID
	rows []comment.ReactionRow
}

func (r *fakeRepo) ApplyCounters(_ context.Context, id uuid.UUID, likes, dislikes int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.applyErr != nil {
		return r.applyErr
	}
	r.counterCalls = append(r.counterCalls, counterCall{id, likes, dislikes})
	return nil
}

func (r *fakeRepo) UpsertReactions(_ context.Context, id uuid.UUID, rows []comment.ReactionRow) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.reactionCalls = append(r.reactionCalls, reactionCall{id, append([]comment.ReactionRow(nil), rows...)})
	return nil
}

// unused interface methods; flusher only calls ApplyCounters and UpsertReactions.
func (r *fakeRepo) InsertRoot(context.Context, comment.Comment) (comment.Comment, error) {
	return comment.Comment{}, errors.New("not used")
}
func (r *fakeRepo) InsertReply(context.Context, comment.Comment) (comment.Comment, error) {
	return comment.Comment{}, errors.New("not used")
}
func (r *fakeRepo) GetByID(context.Context, uuid.UUID) (comment.Comment, error) {
	return comment.Comment{}, errors.New("not used")
}
func (r *fakeRepo) ListRoots(context.Context, comment.TargetType, uuid.UUID, comment.SortOrder, string, int) (comment.RootPage, error) {
	return comment.RootPage{}, errors.New("not used")
}
func (r *fakeRepo) ListReplies(context.Context, uuid.UUID, string, int) (comment.ReplyPage, error) {
	return comment.ReplyPage{}, errors.New("not used")
}
func (r *fakeRepo) LastReplyRankKey(context.Context, uuid.UUID) (string, error) {
	return "", errors.New("not used")
}
func (r *fakeRepo) CountReplies(context.Context, []uuid.UUID) (map[uuid.UUID]int, error) {
	return nil, errors.New("not used")
}
func (r *fakeRepo) UpdateContent(context.Context, uuid.UUID, string) error {
	return errors.New("not used")
}
func (r *fakeRepo) SoftDelete(context.Context, uuid.UUID) error { return errors.New("not used") }

func TestFlusher_NoOpOnEmptyDirty(t *testing.T) {
	store := cache.NewMemoryReactionStore()
	repo := &fakeRepo{}
	f := NewCommentFlusher(store, repo, CommentFlusherConfig{Interval: time.Second})
	if err := f.FlushNow(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(repo.counterCalls) != 0 {
		t.Errorf("counter calls = %d, want 0", len(repo.counterCalls))
	}
}

func TestFlusher_SyncsCountsAndReactions(t *testing.T) {
	store := cache.NewMemoryReactionStore()
	repo := &fakeRepo{}
	f := NewCommentFlusher(store, repo, CommentFlusherConfig{})

	cID := uuid.New()
	u1 := uuid.New()
	_, _ = store.Apply(context.Background(), cID, u1, comment.ReactionLike)

	if err := f.FlushNow(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(repo.counterCalls) != 1 {
		t.Fatalf("counter calls = %d, want 1", len(repo.counterCalls))
	}
	if repo.counterCalls[0].likes != 1 || repo.counterCalls[0].dislikes != 0 {
		t.Errorf("counters = %+v, want {likes:1 dislikes:0}", repo.counterCalls[0])
	}
	if len(repo.reactionCalls) != 1 {
		t.Fatalf("reaction calls = %d, want 1", len(repo.reactionCalls))
	}
	if len(repo.reactionCalls[0].rows) != 1 || repo.reactionCalls[0].rows[0].UserID != u1 {
		t.Errorf("reaction rows = %+v, want one row for %v", repo.reactionCalls[0].rows, u1)
	}
}

func TestFlusher_IdempotentRepeatNoop(t *testing.T) {
	store := cache.NewMemoryReactionStore()
	repo := &fakeRepo{}
	f := NewCommentFlusher(store, repo, CommentFlusherConfig{})

	cID := uuid.New()
	_, _ = store.Apply(context.Background(), cID, uuid.New(), comment.ReactionLike)
	_ = f.FlushNow(context.Background())
	// Second flush should be a no-op because dirty set was drained.
	repo.mu.Lock()
	beforeCount := len(repo.counterCalls)
	repo.mu.Unlock()
	_ = f.FlushNow(context.Background())
	repo.mu.Lock()
	afterCount := len(repo.counterCalls)
	repo.mu.Unlock()
	if afterCount != beforeCount {
		t.Errorf("second flush wrote %d more calls, want 0", afterCount-beforeCount)
	}
}

func TestFlusher_FailedUpsertRequeues(t *testing.T) {
	store := cache.NewMemoryReactionStore()
	repo := &fakeRepo{upsertErr: errors.New("boom")}
	f := NewCommentFlusher(store, repo, CommentFlusherConfig{})

	cID := uuid.New()
	_, _ = store.Apply(context.Background(), cID, uuid.New(), comment.ReactionLike)
	_ = f.FlushNow(context.Background())

	// The failed ID should have been re-marked dirty.
	drained, _ := store.DrainDirty(context.Background(), 10)
	if len(drained) != 1 || drained[0] != cID {
		t.Errorf("after failure drained = %v, want [%v]", drained, cID)
	}
}

func TestFlusher_BootstrapDrainsBeforeTick(t *testing.T) {
	store := cache.NewMemoryReactionStore()
	repo := &fakeRepo{}
	// Pre-load dirty before Start so bootstrap pass picks it up.
	cID := uuid.New()
	_, _ = store.Apply(context.Background(), cID, uuid.New(), comment.ReactionLike)

	f := NewCommentFlusher(store, repo, CommentFlusherConfig{Interval: 30 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	f.Start(ctx)
	defer cancel()
	defer f.Stop()

	// Poll briefly for the bootstrap flush to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		repo.mu.Lock()
		count := len(repo.counterCalls)
		repo.mu.Unlock()
		if count >= 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("bootstrap flush did not run within 2s")
}
