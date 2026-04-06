package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StageCheckpoint holds serialized intermediate data at a stage boundary.
// Version field enables safe schema evolution: if the format changes,
// old checkpoints will fail version check and trigger a fresh run.
type StageCheckpoint struct {
	Version int             `json:"v"`
	Stage   int             `json:"stage"`
	Data    json.RawMessage `json:"data"`
}

// checkpointVersion is incremented when checkpoint data format changes.
const checkpointVersion = 1

// Stage0Data is the checkpoint after Stage 0 completes.
// brainCategories and categories are NOT stored here — they are
// re-fetched from DB on resume (they rarely change).
type Stage0Data struct {
	FormattedText string `json:"formatted_text"`
}

// Stage1Data is the checkpoint after Stage 1 (AI clustering) completes.
// Contains topics and raw Phase 1 JSON (needed for failRun error reporting).
type Stage1Data struct {
	Topics        []Phase1Topic `json:"topics"`
	Phase1RawJSON string        `json:"phase1_raw_json"`
}

// Stage3Data is the checkpoint after Stages 2+3 (URL decode + article fetch).
// Stage 2 is fast and stateless, so it is bundled with Stage 3.
type Stage3Data struct {
	Topics        []Phase1Topic            `json:"topics"`
	ArticleMap    map[int][]FetchedArticle `json:"article_map"`
	Phase1RawJSON string                   `json:"phase1_raw_json"`
}

// CheckpointRepository provides optional checkpoint/resume capabilities.
// It is a separate interface from Repository so existing mocks are unaffected.
// The Service accepts it via WithCheckpointRepo (same pattern as WithQuizRepo).
type CheckpointRepository interface {
	SaveCheckpoint(ctx context.Context, runID uuid.UUID, stage int, data json.RawMessage) error
	GetLatestResumableRun(ctx context.Context, maxAge time.Duration) (*CollectionRun, *int, json.RawMessage, error)
	ClearCheckpoint(ctx context.Context, runID uuid.UUID) error
	CreateRunIfIdle(ctx context.Context, run CollectionRun) (bool, error)
	CleanupOldCheckpoints(ctx context.Context) (int, error)
}

// marshalCheckpoint wraps stage-specific data in a versioned StageCheckpoint envelope.
func marshalCheckpoint(stage int, data any) (json.RawMessage, error) {
	inner, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(StageCheckpoint{
		Version: checkpointVersion,
		Stage:   stage,
		Data:    inner,
	})
}

// unmarshalCheckpoint unwraps the versioned envelope and returns the stage number and raw inner data.
// Returns an error if the version doesn't match.
func unmarshalCheckpoint(raw json.RawMessage) (int, json.RawMessage, error) {
	var cp StageCheckpoint
	if err := json.Unmarshal(raw, &cp); err != nil {
		return 0, nil, err
	}
	if cp.Version != checkpointVersion {
		return 0, nil, &CheckpointVersionError{Got: cp.Version, Want: checkpointVersion}
	}
	return cp.Stage, cp.Data, nil
}

// CheckpointVersionError indicates a checkpoint format version mismatch.
type CheckpointVersionError struct {
	Got  int
	Want int
}

func (e *CheckpointVersionError) Error() string {
	return fmt.Sprintf("checkpoint version mismatch: got %d, want %d", e.Got, e.Want)
}
