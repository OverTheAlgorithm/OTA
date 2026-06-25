package communitytrend

import (
	"context"
	"sort"
	"time"
)

// DailyTagCount is one (tag, date) aggregate row, joined with tag metadata.
type DailyTagCount struct {
	TagID     int       `json:"tag_id"`
	TagName   string    `json:"tag_name"`
	AxisKey   string    `json:"axis_key"`
	StatDate  time.Time `json:"stat_date"`
	PostCount float64   `json:"post_count"`
}

// TrendPoint is a single day's count within a tag's series.
type TrendPoint struct {
	StatDate  time.Time `json:"stat_date"`
	PostCount float64   `json:"post_count"`
}

// TagTrend is a tag's time series plus computed deltas, ready for graphing.
type TagTrend struct {
	TagID         int          `json:"tag_id"`
	TagName       string       `json:"tag_name"`
	AxisKey       string       `json:"axis_key"`
	Points        []TrendPoint `json:"points"`
	Latest        float64      `json:"latest"`          // count on the most recent day in range
	DeltaPrevDay  float64      `json:"delta_prev_day"`  // latest − previous day
	DeltaPrevWeek float64      `json:"delta_prev_week"` // latest − 7 days before latest
}

// AggregateRepository reads daily tag aggregates for a community or cohort.
type AggregateRepository interface {
	// CommunitySeries returns daily tag counts for one community over [from,to].
	CommunitySeries(ctx context.Context, communityID int, from, to time.Time) ([]DailyTagCount, error)
	// CohortSeries sums daily tag counts across communities carrying the given
	// meta tag (the cohort dimension, decisions.md D-010) over [from,to].
	CohortSeries(ctx context.Context, metaTagID int, from, to time.Time) ([]DailyTagCount, error)
}

// AggregateService groups flat rows into per-tag trends and computes deltas,
// applying the conservative surface threshold (CT_MIN_TAG_COUNT, D-002).
type AggregateService struct {
	repo     AggregateRepository
	minCount int
}

func NewAggregateService(repo AggregateRepository, minCount int) *AggregateService {
	return &AggregateService{repo: repo, minCount: minCount}
}

func (s *AggregateService) CommunityTrends(ctx context.Context, communityID int, from, to time.Time) ([]TagTrend, error) {
	rows, err := s.repo.CommunitySeries(ctx, communityID, from, to)
	if err != nil {
		return nil, err
	}
	return s.build(rows, to), nil
}

func (s *AggregateService) CohortTrends(ctx context.Context, metaTagID int, from, to time.Time) ([]TagTrend, error) {
	rows, err := s.repo.CohortSeries(ctx, metaTagID, from, to)
	if err != nil {
		return nil, err
	}
	return s.build(rows, to), nil
}

// build groups rows by tag, computes deltas, and drops tags whose latest count
// is below the conservative threshold.
func (s *AggregateService) build(rows []DailyTagCount, latestDate time.Time) []TagTrend {
	type acc struct {
		name    string
		axisKey string
		byDate  map[string]float64
	}
	grouped := map[int]*acc{}
	for _, r := range rows {
		a := grouped[r.TagID]
		if a == nil {
			a = &acc{name: r.TagName, axisKey: r.AxisKey, byDate: map[string]float64{}}
			grouped[r.TagID] = a
		}
		a.byDate[r.StatDate.Format("2006-01-02")] += r.PostCount
	}

	latestKey := latestDate.Format("2006-01-02")
	prevDayKey := latestDate.AddDate(0, 0, -1).Format("2006-01-02")
	prevWeekKey := latestDate.AddDate(0, 0, -7).Format("2006-01-02")

	var out []TagTrend
	for tagID, a := range grouped {
		latest := a.byDate[latestKey]
		if latest < float64(s.minCount) {
			continue // conservative surface filter
		}
		dates := make([]string, 0, len(a.byDate))
		for d := range a.byDate {
			dates = append(dates, d)
		}
		sort.Strings(dates)
		points := make([]TrendPoint, 0, len(dates))
		for _, d := range dates {
			t, _ := time.Parse("2006-01-02", d)
			points = append(points, TrendPoint{StatDate: t, PostCount: a.byDate[d]})
		}
		out = append(out, TagTrend{
			TagID:         tagID,
			TagName:       a.name,
			AxisKey:       a.axisKey,
			Points:        points,
			Latest:        latest,
			DeltaPrevDay:  latest - a.byDate[prevDayKey],
			DeltaPrevWeek: latest - a.byDate[prevWeekKey],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Latest != out[j].Latest {
			return out[i].Latest > out[j].Latest // hottest first
		}
		return out[i].TagID < out[j].TagID
	})
	return out
}
