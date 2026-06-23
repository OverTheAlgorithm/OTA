package integration

import (
	"context"
	"testing"
)

func TestCommunityTrend_Migration(t *testing.T) {
	db := SetupTestDB(t)
	ctx := context.Background()

	// 14개 ct_ 테이블이 모두 존재하는지
	wantTables := []string{
		"ct_axes", "ct_tags", "ct_communities", "ct_community_tags",
		"ct_tag_daily", "ct_community_daily", "ct_worksheets",
		"ct_robots_status", "ct_robots_transitions", "ct_seen_posts",
		"ct_memes", "ct_meme_candidates", "ct_meme_blacklist", "ct_meme_daily",
	}
	for _, tbl := range wantTables {
		var exists bool
		err := db.Pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, tbl).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", tbl, err)
		}
		if !exists {
			t.Fatalf("expected table %s to exist", tbl)
		}
	}

	// 시드 검증
	var axisCount, commCount, tagCount, attachCount int
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_axes`).Scan(&axisCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_communities`).Scan(&commCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_tags`).Scan(&tagCount)
	db.Pool.QueryRow(ctx, `SELECT count(*) FROM ct_community_tags`).Scan(&attachCount)

	if axisCount != 5 {
		t.Fatalf("expected 5 seeded axes, got %d", axisCount)
	}
	if commCount != 4 {
		t.Fatalf("expected 4 seeded communities, got %d", commCount)
	}
	if tagCount != 6 {
		t.Fatalf("expected 6 seeded meta tags, got %d", tagCount)
	}
	if attachCount < 4 {
		t.Fatalf("expected at least 4 meta-tag attachments, got %d", attachCount)
	}
}
