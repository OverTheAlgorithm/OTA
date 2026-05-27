package handler_test

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ota/domain/editor"
)

// inMemoryEditorRepo is a thread-safe editor.Repository for handler tests.
type inMemoryEditorRepo struct {
	mu    sync.Mutex
	posts map[string]editor.Post
	seq   int
}

func newInMemoryEditorRepo() *inMemoryEditorRepo {
	return &inMemoryEditorRepo{posts: make(map[string]editor.Post)}
}

func (r *inMemoryEditorRepo) Create(_ context.Context, p editor.Post) (editor.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	p.ID = "post-" + strconv.Itoa(r.seq)
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	r.posts[p.ID] = p
	return p, nil
}

func (r *inMemoryEditorRepo) Update(_ context.Context, p editor.Post) (editor.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.posts[p.ID]; !ok {
		return editor.Post{}, editor.ErrPostNotFound
	}
	p.UpdatedAt = time.Now()
	r.posts[p.ID] = p
	return p, nil
}

func (r *inMemoryEditorRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.posts[id]; !ok {
		return editor.ErrPostNotFound
	}
	delete(r.posts, id)
	return nil
}

func (r *inMemoryEditorRepo) FindByID(_ context.Context, id string) (editor.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.posts[id]; ok {
		return p, nil
	}
	return editor.Post{}, editor.ErrPostNotFound
}

func (r *inMemoryEditorRepo) FindDraftByAuthor(_ context.Context, authorID string) (editor.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.posts {
		if p.AuthorID == authorID && p.Status == editor.StatusDraft {
			return p, nil
		}
	}
	return editor.Post{}, editor.ErrPostNotFound
}

func (r *inMemoryEditorRepo) ListByAuthor(_ context.Context, authorID string) ([]editor.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []editor.Post
	for _, p := range r.posts {
		if p.AuthorID == authorID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *inMemoryEditorRepo) ListAllForAdmin(_ context.Context) ([]editor.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]editor.Post, 0, len(r.posts))
	for _, p := range r.posts {
		out = append(out, p)
	}
	return out, nil
}

func (r *inMemoryEditorRepo) ListPublishedCards(_ context.Context, limit, offset int) ([]editor.PublicCard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []editor.PublicCard
	for _, p := range r.posts {
		if p.Status != editor.StatusPublished || p.PublishedAt == nil {
			continue
		}
		all = append(all, editor.PublicCard{
			ID:            p.ID,
			AuthorID:      p.AuthorID,
			Title:         p.Title,
			Excerpt:       p.ContentText,
			FirstImageURL: p.FirstImageURL,
			PublishedAt:   *p.PublishedAt,
		})
	}
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *inMemoryEditorRepo) GetPublishedByID(_ context.Context, id string) (editor.PublicPost, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.posts[id]
	if !ok || p.Status != editor.StatusPublished || p.PublishedAt == nil {
		return editor.PublicPost{}, editor.ErrPostNotFound
	}
	return editor.PublicPost{
		ID:            p.ID,
		Title:         p.Title,
		ContentHTML:   p.ContentHTML,
		FirstImageURL: p.FirstImageURL,
		AuthorID:      p.AuthorID,
		PublishedAt:   *p.PublishedAt,
		UpdatedAt:     p.UpdatedAt,
	}, nil
}

func (r *inMemoryEditorRepo) CountPublished(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, p := range r.posts {
		if p.Status == editor.StatusPublished {
			n++
		}
	}
	return n, nil
}

// SearchPublishedCards mirrors the storage implementation's ranking — title
// matches outrank body matches, ties break by recency — so handler tests can
// exercise the contract without spinning up a Postgres container.
func (r *inMemoryEditorRepo) SearchPublishedCards(_ context.Context, query string, limit, offset int) ([]editor.PublicCard, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	needle := strings.ToLower(query)
	type ranked struct {
		card editor.PublicCard
		rank int
	}
	var hits []ranked
	for _, p := range r.posts {
		if p.Status != editor.StatusPublished || p.PublishedAt == nil {
			continue
		}
		titleHit := strings.Contains(strings.ToLower(p.Title), needle)
		bodyHit := strings.Contains(strings.ToLower(p.ContentText), needle)
		if !titleHit && !bodyHit {
			continue
		}
		rank := 1
		if titleHit {
			rank = 2
		}
		hits = append(hits, ranked{
			card: editor.PublicCard{
				ID:            p.ID,
				AuthorID:      p.AuthorID,
				Title:         p.Title,
				Excerpt:       p.ContentText,
				FirstImageURL: p.FirstImageURL,
				PublishedAt:   *p.PublishedAt,
			},
			rank: rank,
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank > hits[j].rank
		}
		return hits[i].card.PublishedAt.After(hits[j].card.PublishedAt)
	})

	hasMore := false
	if offset >= len(hits) {
		return nil, false, nil
	}
	end := offset + limit + 1
	if end > len(hits) {
		end = len(hits)
	} else {
		hasMore = true
	}
	picked := hits[offset:end]
	if hasMore {
		picked = picked[:limit]
	}
	out := make([]editor.PublicCard, len(picked))
	for i, p := range picked {
		out[i] = p.card
	}
	return out, hasMore, nil
}

// errNotUsed is here to satisfy any future linter complaint about the unused errors import.
var _ = errors.New
