package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/comment"
	"ota/storage"
)

// seedUser inserts a user row and returns its id.
func seedUser(t *testing.T, db *TestDB, nickname, penName string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := db.Pool.Exec(ctx, `
        INSERT INTO users (id, kakao_id, email, nickname, pen_name)
        VALUES ($1, $2, $3, $4, $5)`,
		id, time.Now().UnixNano(), nickname+"@example.com", nickname, nullable(penName),
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func TestCommentRepository_InsertAndGetRoot(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()

	id := uuid.New()
	target := uuid.New()
	in := comment.Comment{
		ID:         id,
		TargetType: comment.TargetTopic,
		TargetID:   target,
		UserID:     uID,
		GroupID:    id,
		Depth:      0,
		RankKey:    comment.First(),
		Content:    "hello world",
	}
	saved, err := repo.InsertRoot(ctx, in)
	if err != nil {
		t.Fatalf("insert root: %v", err)
	}
	if saved.ID != id || saved.Depth != 0 || saved.GroupID != id {
		t.Errorf("saved comment fields wrong: %+v", saved)
	}
	if saved.AuthorNickname != "alice" {
		t.Errorf("author nickname = %q, want alice", saved.AuthorNickname)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Content != "hello world" {
		t.Errorf("content = %q, want hello world", got.Content)
	}
}

// Pen names are deliberately not surfaced through the comment join — even
// when an editor has set a pen name for their byline, comments always show
// the nickname.
func TestCommentRepository_PenNameNotSurfaced(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "bob", "TheBuilder")
	ctx := context.Background()

	id := uuid.New()
	target := uuid.New()
	in := comment.Comment{
		ID: id, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: id, Depth: 0, RankKey: comment.First(),
		Content: "x",
	}
	saved, err := repo.InsertRoot(ctx, in)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if saved.AuthorNickname != "bob" {
		t.Errorf("nickname = %q, want bob", saved.AuthorNickname)
	}
	if saved.AuthorDisplayName() != "bob" {
		t.Errorf("display = %q, want bob (not pen name)", saved.AuthorDisplayName())
	}
}

func TestCommentRepository_InsertReplyAndLastRank(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()

	// Root
	rootID := uuid.New()
	root := comment.Comment{
		ID: rootID, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: rootID, Depth: 0, RankKey: comment.First(),
		Content: "root",
	}
	if _, err := repo.InsertRoot(ctx, root); err != nil {
		t.Fatalf("insert root: %v", err)
	}

	// Last rank in empty group is "".
	last, err := repo.LastReplyRankKey(ctx, rootID)
	if err != nil {
		t.Fatalf("last rank: %v", err)
	}
	if last != "" {
		t.Errorf("empty group last rank = %q, want empty", last)
	}

	// First reply uses comment.First() rank.
	reply1ID := uuid.New()
	r1 := comment.Comment{
		ID: reply1ID, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: rootID, ParentID: &rootID, Depth: 1,
		RankKey: comment.First(), Content: "reply1",
	}
	if _, err := repo.InsertReply(ctx, r1); err != nil {
		t.Fatalf("insert reply1: %v", err)
	}

	// After("U") rank
	rk2, _ := comment.After(comment.First())
	reply2ID := uuid.New()
	r2 := comment.Comment{
		ID: reply2ID, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: rootID, ParentID: &rootID, Depth: 1,
		RankKey: rk2, Content: "reply2",
	}
	if _, err := repo.InsertReply(ctx, r2); err != nil {
		t.Fatalf("insert reply2: %v", err)
	}

	last, err = repo.LastReplyRankKey(ctx, rootID)
	if err != nil {
		t.Fatalf("last rank: %v", err)
	}
	if last != rk2 {
		t.Errorf("last rank = %q, want %q", last, rk2)
	}
}

func TestCommentRepository_ListRootsByPopular(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()

	// Insert 3 roots with different likes.
	ids := make([]uuid.UUID, 0, 3)
	for i, likes := range []int{0, 5, 2} {
		id := uuid.New()
		ids = append(ids, id)
		c := comment.Comment{
			ID: id, TargetType: comment.TargetTopic, TargetID: target,
			UserID: uID, GroupID: id, Depth: 0, RankKey: comment.First(),
			Content: "c" + string(rune('0'+i)),
		}
		if _, err := repo.InsertRoot(ctx, c); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if err := repo.ApplyCounters(ctx, id, likes, 0); err != nil {
			t.Fatalf("apply counters: %v", err)
		}
	}

	page, err := repo.ListRoots(ctx, comment.TargetTopic, target, comment.SortPopular, "", 10)
	if err != nil {
		t.Fatalf("list roots: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(page.Items))
	}
	if page.Items[0].LikesCount != 5 || page.Items[1].LikesCount != 2 || page.Items[2].LikesCount != 0 {
		t.Errorf("ordering wrong: %v %v %v",
			page.Items[0].LikesCount, page.Items[1].LikesCount, page.Items[2].LikesCount)
	}
}

func TestCommentRepository_ListRootsPagination(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()

	// Insert 5 roots; assign descending likes so order is deterministic.
	for i := 4; i >= 0; i-- {
		id := uuid.New()
		c := comment.Comment{
			ID: id, TargetType: comment.TargetTopic, TargetID: target,
			UserID: uID, GroupID: id, Depth: 0, RankKey: comment.First(),
			Content: "c",
		}
		if _, err := repo.InsertRoot(ctx, c); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if err := repo.ApplyCounters(ctx, id, i, 0); err != nil {
			t.Fatalf("apply counters: %v", err)
		}
	}

	// First page of 2.
	p1, err := repo.ListRoots(ctx, comment.TargetTopic, target, comment.SortPopular, "", 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(p1.Items) != 2 {
		t.Fatalf("page1 items = %d, want 2", len(p1.Items))
	}
	if p1.NextCursor == "" {
		t.Error("page1 NextCursor empty, expected cursor")
	}

	// Second page.
	p2, err := repo.ListRoots(ctx, comment.TargetTopic, target, comment.SortPopular, p1.NextCursor, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(p2.Items) != 2 {
		t.Fatalf("page2 items = %d, want 2", len(p2.Items))
	}
	// No duplicates across pages.
	for _, a := range p1.Items {
		for _, b := range p2.Items {
			if a.ID == b.ID {
				t.Errorf("duplicate id %v across pages", a.ID)
			}
		}
	}

	// Third page (last one item, no cursor).
	p3, err := repo.ListRoots(ctx, comment.TargetTopic, target, comment.SortPopular, p2.NextCursor, 2)
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if len(p3.Items) != 1 {
		t.Fatalf("page3 items = %d, want 1", len(p3.Items))
	}
	if p3.NextCursor != "" {
		t.Errorf("page3 NextCursor = %q, want empty for last page", p3.NextCursor)
	}
}

func TestCommentRepository_ListRepliesByRank(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()

	rootID := uuid.New()
	root := comment.Comment{
		ID: rootID, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: rootID, Depth: 0, RankKey: comment.First(),
		Content: "root",
	}
	if _, err := repo.InsertRoot(ctx, root); err != nil {
		t.Fatalf("insert root: %v", err)
	}

	// Insert replies with deliberately scrambled rank_keys so we can prove
	// ordering is by rank, not insertion order.
	ranks := []string{"U", "V", "M"} // M < U < V
	for i, rk := range ranks {
		r := comment.Comment{
			ID: uuid.New(), TargetType: comment.TargetTopic, TargetID: target,
			UserID: uID, GroupID: rootID, ParentID: &rootID, Depth: 1,
			RankKey: rk, Content: "r" + string(rune('0'+i)),
		}
		if _, err := repo.InsertReply(ctx, r); err != nil {
			t.Fatalf("insert reply: %v", err)
		}
	}

	page, err := repo.ListReplies(ctx, rootID, "", 10)
	if err != nil {
		t.Fatalf("list replies: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(page.Items))
	}
	gotRanks := []string{page.Items[0].RankKey, page.Items[1].RankKey, page.Items[2].RankKey}
	wantRanks := []string{"M", "U", "V"}
	for i := range gotRanks {
		if gotRanks[i] != wantRanks[i] {
			t.Errorf("rank[%d] = %q, want %q", i, gotRanks[i], wantRanks[i])
		}
	}
}

func TestCommentRepository_CountReplies(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()

	rootID := uuid.New()
	root := comment.Comment{
		ID: rootID, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: rootID, Depth: 0, RankKey: comment.First(),
		Content: "root",
	}
	if _, err := repo.InsertRoot(ctx, root); err != nil {
		t.Fatalf("insert root: %v", err)
	}

	// 3 replies.
	rk := comment.First()
	for i := 0; i < 3; i++ {
		r := comment.Comment{
			ID: uuid.New(), TargetType: comment.TargetTopic, TargetID: target,
			UserID: uID, GroupID: rootID, ParentID: &rootID, Depth: 1,
			RankKey: rk, Content: "r",
		}
		if _, err := repo.InsertReply(ctx, r); err != nil {
			t.Fatalf("insert reply: %v", err)
		}
		rk, _ = comment.After(rk)
	}

	counts, err := repo.CountReplies(ctx, []uuid.UUID{rootID})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if counts[rootID] != 3 {
		t.Errorf("count = %d, want 3", counts[rootID])
	}
}

func TestCommentRepository_UpdateContentTouchesEditedAt(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()
	id := uuid.New()
	c := comment.Comment{
		ID: id, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: id, Depth: 0, RankKey: comment.First(),
		Content: "original",
	}
	if _, err := repo.InsertRoot(ctx, c); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := repo.UpdateContent(ctx, id, "edited"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "edited" {
		t.Errorf("content = %q, want edited", got.Content)
	}
	if got.EditedAt == nil {
		t.Error("EditedAt is nil after update")
	}
}

func TestCommentRepository_SoftDeleteHidesContent(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	ctx := context.Background()
	target := uuid.New()
	id := uuid.New()
	c := comment.Comment{
		ID: id, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: id, Depth: 0, RankKey: comment.First(),
		Content: "secret",
	}
	if _, err := repo.InsertRoot(ctx, c); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.SoftDelete(ctx, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.IsDeleted() {
		t.Error("comment not marked deleted")
	}
	// Content is preserved in the DB (CHECK constraint forbids empty);
	// the handler is responsible for masking it in responses.
	if got.Content != "secret" {
		t.Errorf("content after soft delete = %q, want preserved 'secret'", got.Content)
	}
}

func TestCommentRepository_UpsertReactionsReconciles(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "comments", "comment_reactions", "users")

	repo := storage.NewCommentRepository(db.Pool)
	uID := seedUser(t, db, "alice", "")
	u2 := seedUser(t, db, "bob", "")
	ctx := context.Background()
	target := uuid.New()
	id := uuid.New()
	c := comment.Comment{
		ID: id, TargetType: comment.TargetTopic, TargetID: target,
		UserID: uID, GroupID: id, Depth: 0, RankKey: comment.First(),
		Content: "x",
	}
	if _, err := repo.InsertRoot(ctx, c); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Initial set: alice likes, bob dislikes.
	rows := []comment.ReactionRow{
		{UserID: uID, Reaction: comment.ReactionLike},
		{UserID: u2, Reaction: comment.ReactionDislike},
	}
	if err := repo.UpsertReactions(ctx, id, rows); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Verify rows are present.
	var aliceR, bobR int
	if err := db.Pool.QueryRow(ctx, `SELECT reaction FROM comment_reactions WHERE comment_id=$1 AND user_id=$2`, id, uID).Scan(&aliceR); err != nil {
		t.Fatalf("read alice: %v", err)
	}
	if aliceR != 1 {
		t.Errorf("alice = %d, want 1", aliceR)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT reaction FROM comment_reactions WHERE comment_id=$1 AND user_id=$2`, id, u2).Scan(&bobR); err != nil {
		t.Fatalf("read bob: %v", err)
	}
	if bobR != -1 {
		t.Errorf("bob = %d, want -1", bobR)
	}

	// Reconciliation: only alice remains, but switched to dislike.
	rows = []comment.ReactionRow{{UserID: uID, Reaction: comment.ReactionDislike}}
	if err := repo.UpsertReactions(ctx, id, rows); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	var aliceAfter int
	if err := db.Pool.QueryRow(ctx, `SELECT reaction FROM comment_reactions WHERE comment_id=$1 AND user_id=$2`, id, uID).Scan(&aliceAfter); err != nil {
		t.Fatalf("read alice2: %v", err)
	}
	if aliceAfter != -1 {
		t.Errorf("alice after reconcile = %d, want -1", aliceAfter)
	}
	var bobCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM comment_reactions WHERE comment_id=$1 AND user_id=$2`, id, u2).Scan(&bobCount); err != nil {
		t.Fatalf("count bob: %v", err)
	}
	if bobCount != 0 {
		t.Errorf("bob row count after reconcile = %d, want 0 (should be deleted)", bobCount)
	}

	// Empty reconciliation removes all.
	if err := repo.UpsertReactions(ctx, id, nil); err != nil {
		t.Fatalf("upsert empty: %v", err)
	}
	var totalAfter int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM comment_reactions WHERE comment_id=$1`, id).Scan(&totalAfter); err != nil {
		t.Fatalf("count: %v", err)
	}
	if totalAfter != 0 {
		t.Errorf("total after empty upsert = %d, want 0", totalAfter)
	}
}
