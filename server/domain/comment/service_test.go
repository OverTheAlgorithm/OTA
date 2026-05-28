package comment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockRepo struct {
	roots         map[uuid.UUID]Comment
	replies       map[uuid.UUID]Comment
	lastRank      map[uuid.UUID]string
	insertRootErr error
	insertReplyErr error
	getByIDErr    error
	updateErr     error
	deleteErr     error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		roots:    make(map[uuid.UUID]Comment),
		replies:  make(map[uuid.UUID]Comment),
		lastRank: make(map[uuid.UUID]string),
	}
}

func (m *mockRepo) InsertRoot(_ context.Context, c Comment) (Comment, error) {
	if m.insertRootErr != nil {
		return Comment{}, m.insertRootErr
	}
	m.roots[c.ID] = c
	return c, nil
}

func (m *mockRepo) InsertReply(_ context.Context, c Comment) (Comment, error) {
	if m.insertReplyErr != nil {
		return Comment{}, m.insertReplyErr
	}
	m.replies[c.ID] = c
	m.lastRank[c.GroupID] = c.RankKey
	return c, nil
}

func (m *mockRepo) GetByID(_ context.Context, id uuid.UUID) (Comment, error) {
	if m.getByIDErr != nil {
		return Comment{}, m.getByIDErr
	}
	if c, ok := m.roots[id]; ok {
		return c, nil
	}
	if c, ok := m.replies[id]; ok {
		return c, nil
	}
	return Comment{}, ErrCommentNotFound
}

func (m *mockRepo) ListRoots(_ context.Context, _ TargetType, _ uuid.UUID, _ SortOrder, _ string, _ int) (RootPage, error) {
	return RootPage{}, nil
}

func (m *mockRepo) ListReplies(_ context.Context, _ uuid.UUID, _ string, _ int) (ReplyPage, error) {
	return ReplyPage{}, nil
}

func (m *mockRepo) LastReplyRankKey(_ context.Context, groupID uuid.UUID) (string, error) {
	return m.lastRank[groupID], nil
}

func (m *mockRepo) CountReplies(_ context.Context, _ []uuid.UUID) (map[uuid.UUID]int, error) {
	return map[uuid.UUID]int{}, nil
}

func (m *mockRepo) UpdateContent(_ context.Context, id uuid.UUID, newContent string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if c, ok := m.roots[id]; ok {
		c.Content = newContent
		m.roots[id] = c
		return nil
	}
	if c, ok := m.replies[id]; ok {
		c.Content = newContent
		m.replies[id] = c
		return nil
	}
	return ErrCommentNotFound
}

func (m *mockRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	now := mustTime()
	if c, ok := m.roots[id]; ok {
		c.DeletedAt = &now
		m.roots[id] = c
		return nil
	}
	if c, ok := m.replies[id]; ok {
		c.DeletedAt = &now
		m.replies[id] = c
		return nil
	}
	return ErrCommentNotFound
}

func (m *mockRepo) ApplyCounters(_ context.Context, _ uuid.UUID, _ int, _ int) error {
	return nil
}

func (m *mockRepo) UpsertReactions(_ context.Context, _ uuid.UUID, _ []ReactionRow) error {
	return nil
}

type mockReactionStore struct {
	applyResult ReactionApplyResult
	applyErr    error
	applyCalls  int
}

func (m *mockReactionStore) Apply(_ context.Context, _, _ uuid.UUID, target Reaction) (ReactionApplyResult, error) {
	m.applyCalls++
	if m.applyErr != nil {
		return ReactionApplyResult{}, m.applyErr
	}
	res := m.applyResult
	res.Current = target
	return res, nil
}

func (m *mockReactionStore) Counts(_ context.Context, _ uuid.UUID) (ReactionCounts, error) {
	return m.applyResult.Counts, nil
}

func (m *mockReactionStore) BatchCounts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]ReactionCounts, error) {
	out := make(map[uuid.UUID]ReactionCounts, len(ids))
	for _, id := range ids {
		out[id] = m.applyResult.Counts
	}
	return out, nil
}

func (m *mockReactionStore) UserReaction(_ context.Context, _, _ uuid.UUID) (Reaction, error) {
	return ReactionNone, nil
}

func (m *mockReactionStore) BatchUserReactions(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Reaction, error) {
	return map[uuid.UUID]Reaction{}, nil
}

func (m *mockReactionStore) Hydrate(_ context.Context, _ uuid.UUID, _ ReactionCounts, _ []ReactionRow) error {
	return nil
}

func (m *mockReactionStore) DrainDirty(_ context.Context, _ int) ([]uuid.UUID, error) {
	return nil, nil
}

func (m *mockReactionStore) MarkDirty(_ context.Context, _ uuid.UUID) error {
	return nil
}

type mockValidator struct {
	exists bool
	err    error
}

func (m mockValidator) Exists(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.exists, m.err
}

func mustTime() time.Time {
	return time.Now().UTC()
}

func newService(repo Repository, reactions ReactionStore) *Service {
	return NewService(repo, reactions, map[TargetType]TargetValidator{
		TargetTopic:      mockValidator{exists: true},
		TargetEditorPick: mockValidator{exists: true},
	})
}

func TestCreate_AnonymousRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.Create(context.Background(), uuid.Nil, TargetTopic, uuid.New(), nil, "hi")
	if !errors.Is(err, ErrAnonymousForbidden) {
		t.Errorf("err = %v, want ErrAnonymousForbidden", err)
	}
}

func TestCreate_InvalidTargetRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.Create(context.Background(), uuid.New(), TargetType("post"), uuid.New(), nil, "hi")
	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("err = %v, want ErrInvalidTarget", err)
	}
}

func TestCreate_EmptyContentRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.Create(context.Background(), uuid.New(), TargetTopic, uuid.New(), nil, "   ")
	if !errors.Is(err, ErrInvalidContent) {
		t.Errorf("err = %v, want ErrInvalidContent", err)
	}
}

func TestCreate_OverLongContentRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	tooLong := strings.Repeat("a", MaxContentLen+1)
	_, err := svc.Create(context.Background(), uuid.New(), TargetTopic, uuid.New(), nil, tooLong)
	if !errors.Is(err, ErrInvalidContent) {
		t.Errorf("err = %v, want ErrInvalidContent", err)
	}
}

func TestCreate_MissingTargetRejected(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockReactionStore{}, map[TargetType]TargetValidator{
		TargetTopic: mockValidator{exists: false},
	})
	_, err := svc.Create(context.Background(), uuid.New(), TargetTopic, uuid.New(), nil, "hi")
	if !errors.Is(err, ErrTargetNotExist) {
		t.Errorf("err = %v, want ErrTargetNotExist", err)
	}
}

func TestCreate_RootSetsGroupIDToOwnID(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	user := uuid.New()
	target := uuid.New()
	got, err := svc.Create(context.Background(), user, TargetTopic, target, nil, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GroupID != got.ID {
		t.Errorf("GroupID = %v, want = ID %v", got.GroupID, got.ID)
	}
	if got.Depth != 0 {
		t.Errorf("Depth = %d, want 0", got.Depth)
	}
	if got.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", got.ParentID)
	}
	if got.RankKey == "" {
		t.Error("RankKey should be set")
	}
}

func TestCreate_ReplyToRoot_Depth1(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	user := uuid.New()
	target := uuid.New()
	root, err := svc.Create(context.Background(), user, TargetTopic, target, nil, "root")
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	reply, err := svc.Create(context.Background(), user, TargetTopic, target, &root.ID, "reply")
	if err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if reply.Depth != 1 {
		t.Errorf("Depth = %d, want 1", reply.Depth)
	}
	if reply.GroupID != root.ID {
		t.Errorf("GroupID = %v, want root.ID %v", reply.GroupID, root.ID)
	}
	if reply.ParentID == nil || *reply.ParentID != root.ID {
		t.Errorf("ParentID = %v, want root.ID %v", reply.ParentID, root.ID)
	}
	if reply.RankKey == "" {
		t.Error("RankKey should be set")
	}
}

func TestCreate_ReplyToReply_CollapsesToSibling(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	user := uuid.New()
	target := uuid.New()
	root, _ := svc.Create(context.Background(), user, TargetTopic, target, nil, "root")
	reply1, _ := svc.Create(context.Background(), user, TargetTopic, target, &root.ID, "reply1")
	reply2, err := svc.Create(context.Background(), user, TargetTopic, target, &reply1.ID, "reply2")
	if err != nil {
		t.Fatalf("create reply2: %v", err)
	}
	if reply2.Depth != 1 {
		t.Errorf("Depth = %d, want 1 (sibling, not 2)", reply2.Depth)
	}
	if reply2.GroupID != root.ID {
		t.Errorf("GroupID = %v, want root.ID %v", reply2.GroupID, root.ID)
	}
	if reply2.ParentID == nil || *reply2.ParentID != root.ID {
		t.Errorf("ParentID = %v, want root.ID (grandparent, not reply1) %v", reply2.ParentID, root.ID)
	}
	if !(reply2.RankKey > reply1.RankKey) {
		t.Errorf("reply2 rank %q must be > reply1 rank %q for chronological order",
			reply2.RankKey, reply1.RankKey)
	}
}

func TestCreate_ReplyMismatchedTargetRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	user := uuid.New()
	target := uuid.New()
	root, _ := svc.Create(context.Background(), user, TargetTopic, target, nil, "root")

	other := uuid.New()
	_, err := svc.Create(context.Background(), user, TargetTopic, other, &root.ID, "should fail")
	if !errors.Is(err, ErrInvalidParent) {
		t.Errorf("err = %v, want ErrInvalidParent", err)
	}
}

func TestUpdate_NonOwnerRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	intruder := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "original")
	_, err := svc.Update(context.Background(), intruder, c.ID, "edited")
	if !errors.Is(err, ErrNotOwner) {
		t.Errorf("err = %v, want ErrNotOwner", err)
	}
}

func TestUpdate_AnonymousRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.Update(context.Background(), uuid.Nil, uuid.New(), "x")
	if !errors.Is(err, ErrAnonymousForbidden) {
		t.Errorf("err = %v, want ErrAnonymousForbidden", err)
	}
}

func TestUpdate_DeletedRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "original")
	_ = svc.Delete(context.Background(), owner, c.ID)
	_, err := svc.Update(context.Background(), owner, c.ID, "edited")
	if !errors.Is(err, ErrAlreadyDeleted) {
		t.Errorf("err = %v, want ErrAlreadyDeleted", err)
	}
}

func TestUpdate_InvalidContentRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "original")
	_, err := svc.Update(context.Background(), owner, c.ID, "  ")
	if !errors.Is(err, ErrInvalidContent) {
		t.Errorf("err = %v, want ErrInvalidContent", err)
	}
}

func TestDelete_NonOwnerRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	intruder := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "original")
	err := svc.Delete(context.Background(), intruder, c.ID)
	if !errors.Is(err, ErrNotOwner) {
		t.Errorf("err = %v, want ErrNotOwner", err)
	}
}

func TestAdminDelete_BypassesOwnership(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "original")
	if err := svc.AdminDelete(context.Background(), c.ID); err != nil {
		t.Errorf("admin delete err = %v", err)
	}
}

func TestReact_InvalidValueRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "x")
	_, err := svc.React(context.Background(), owner, c.ID, Reaction(2))
	if !errors.Is(err, ErrInvalidReaction) {
		t.Errorf("err = %v, want ErrInvalidReaction", err)
	}
}

func TestReact_AnonymousRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.React(context.Background(), uuid.Nil, uuid.New(), ReactionLike)
	if !errors.Is(err, ErrAnonymousForbidden) {
		t.Errorf("err = %v, want ErrAnonymousForbidden", err)
	}
}

func TestReact_DeletedCommentRejected(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	owner := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "x")
	_ = svc.Delete(context.Background(), owner, c.ID)
	_, err := svc.React(context.Background(), owner, c.ID, ReactionLike)
	if !errors.Is(err, ErrAlreadyDeleted) {
		t.Errorf("err = %v, want ErrAlreadyDeleted", err)
	}
}

func TestReact_DelegatesToStore(t *testing.T) {
	repo := newMockRepo()
	store := &mockReactionStore{applyResult: ReactionApplyResult{Previous: ReactionNone, Counts: ReactionCounts{Likes: 1}}}
	svc := newService(repo, store)
	owner := uuid.New()
	target := uuid.New()
	c, _ := svc.Create(context.Background(), owner, TargetTopic, target, nil, "x")

	res, err := svc.React(context.Background(), owner, c.ID, ReactionLike)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.applyCalls != 1 {
		t.Errorf("apply calls = %d, want 1", store.applyCalls)
	}
	if res.Counts.Likes != 1 {
		t.Errorf("result likes = %d, want 1", res.Counts.Likes)
	}
}

func TestListRoots_DefaultsSortAndLimit(t *testing.T) {
	repo := newMockRepo()
	svc := newService(repo, &mockReactionStore{})
	_, err := svc.ListRoots(context.Background(), TargetTopic, uuid.New(), SortOrder("garbage"), "", 0)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestListRoots_InvalidTargetRejected(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.ListRoots(context.Background(), TargetType("post"), uuid.New(), SortPopular, "", 10)
	if !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("err = %v, want ErrInvalidTarget", err)
	}
}

func TestListReplies_RequiresGroupID(t *testing.T) {
	svc := newService(newMockRepo(), &mockReactionStore{})
	_, err := svc.ListReplies(context.Background(), uuid.Nil, "", 10)
	if !errors.Is(err, ErrCommentNotFound) {
		t.Errorf("err = %v, want ErrCommentNotFound", err)
	}
}
