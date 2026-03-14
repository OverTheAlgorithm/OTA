package collector

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateRun(ctx context.Context, run CollectionRun) error
	CompleteRun(ctx context.Context, id uuid.UUID, status RunStatus, errMsg *string, rawResponse *string) error
	SaveContextItems(ctx context.Context, items []ContextItem) error
	UpdateItemImagePath(ctx context.Context, itemID uuid.UUID, imagePath string) error
	CanRunToday(ctx context.Context) (bool, error)
}
