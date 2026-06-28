package communitytrend

import (
	"context"
	"testing"
	"time"
)

type fakeAggRepo struct{ rows []DailyTagCount }

func (f *fakeAggRepo) CommunitySeries(context.Context, int, time.Time, time.Time) ([]DailyTagCount, error) {
	return f.rows, nil
}
func (f *fakeAggRepo) CohortSeries(context.Context, int, time.Time, time.Time) ([]DailyTagCount, error) {
	return f.rows, nil
}

func d(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestAggregate_DeltasAndThreshold(t *testing.T) {
	to := d("2026-06-24")
	rows := []DailyTagCount{
		// tag 1 "남성 인권": rising
		{TagID: 1, TagName: "남성 인권", AxisKey: "gender_topic", StatDate: d("2026-06-17"), PostCount: 1}, // 7 days before
		{TagID: 1, TagName: "남성 인권", AxisKey: "gender_topic", StatDate: d("2026-06-23"), PostCount: 3}, // prev day
		{TagID: 1, TagName: "남성 인권", AxisKey: "gender_topic", StatDate: d("2026-06-24"), PostCount: 5}, // latest
		// tag 2 below threshold (latest 2 < 3) → filtered
		{TagID: 2, TagName: "노잼", AxisKey: "x", StatDate: d("2026-06-24"), PostCount: 2},
	}
	svc := NewAggregateService(&fakeAggRepo{rows: rows}, 3.0)

	trends, err := svc.CommunityTrends(context.Background(), 1, d("2026-06-17"), to)
	if err != nil {
		t.Fatalf("trends: %v", err)
	}
	if len(trends) != 1 {
		t.Fatalf("expected 1 tag above threshold, got %d", len(trends))
	}
	tt := trends[0]
	if tt.TagID != 1 || tt.Latest != 5 {
		t.Fatalf("unexpected trend: %+v", tt)
	}
	if tt.DeltaPrevDay != 2.0 { // 5 - 3
		t.Fatalf("DeltaPrevDay = %f, want 2.0", tt.DeltaPrevDay)
	}
	if tt.DeltaPrevWeek != 4.0 { // 5 - 1
		t.Fatalf("DeltaPrevWeek = %f, want 4.0", tt.DeltaPrevWeek)
	}
	if len(tt.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(tt.Points))
	}
	// points sorted ascending by date
	if !tt.Points[0].StatDate.Equal(d("2026-06-17")) || !tt.Points[2].StatDate.Equal(to) {
		t.Fatalf("points not sorted: %+v", tt.Points)
	}
}

func TestAggregate_CohortSumsAcrossCommunities(t *testing.T) {
	to := d("2026-06-24")
	// same tag/date from two communities → summed
	rows := []DailyTagCount{
		{TagID: 9, TagName: "우파 지지", AxisKey: "political_topic", StatDate: to, PostCount: 4},
		{TagID: 9, TagName: "우파 지지", AxisKey: "political_topic", StatDate: to, PostCount: 6},
	}
	svc := NewAggregateService(&fakeAggRepo{rows: rows}, 3.0)
	trends, err := svc.CohortTrends(context.Background(), 100, to, to)
	if err != nil {
		t.Fatalf("cohort: %v", err)
	}
	if len(trends) != 1 || trends[0].Latest != 10 {
		t.Fatalf("expected summed latest 10, got %+v", trends)
	}
}
