package communitytrend

import "context"

// AxisRepository persists tag axes.
type AxisRepository interface {
	Create(ctx context.Context, a Axis) (Axis, error)
	List(ctx context.Context) ([]Axis, error)
	Delete(ctx context.Context, id int) error
}

// TagRepository persists the shared tag pool.
type TagRepository interface {
	Create(ctx context.Context, t Tag) (Tag, error)
	List(ctx context.Context) ([]Tag, error)
	ListByAxis(ctx context.Context, axisID int) ([]Tag, error)
	Update(ctx context.Context, id int, name, description string) (Tag, error)
	Delete(ctx context.Context, id int) error
}

// CommunityRepository persists communities and their meta-tag attachments.
type CommunityRepository interface {
	Create(ctx context.Context, c Community) (Community, error)
	List(ctx context.Context) ([]Community, error)
	Update(ctx context.Context, id int, name, homeURL string, enabled bool) (Community, error)
	Delete(ctx context.Context, id int) error
	// SetMetaTags replaces the full meta-tag set for a community atomically.
	SetMetaTags(ctx context.Context, communityID int, tagIDs []int) error
	GetMetaTags(ctx context.Context, communityID int) ([]int, error)
}
