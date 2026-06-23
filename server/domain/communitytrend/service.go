package communitytrend

import (
	"context"
	"fmt"
	"regexp"
)

// keyPattern enforces adapter-linkable community keys: lower alnum, dash, underscore.
var keyPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// Service holds validation and orchestration over the repositories.
type Service struct {
	communities CommunityRepository
	tags        TagRepository
	axes        AxisRepository
}

func NewService(communities CommunityRepository, tags TagRepository, axes AxisRepository) *Service {
	return &Service{communities: communities, tags: tags, axes: axes}
}

// --- communities ---

func (s *Service) CreateCommunity(ctx context.Context, c Community) (Community, error) {
	if !keyPattern.MatchString(c.Key) {
		return Community{}, fmt.Errorf("커뮤니티 key는 소문자 영숫자/-/_ 만 허용됩니다")
	}
	if c.Name == "" {
		return Community{}, fmt.Errorf("커뮤니티 이름은 필수입니다")
	}
	return s.communities.Create(ctx, c)
}

func (s *Service) ListCommunities(ctx context.Context) ([]Community, error) {
	list, err := s.communities.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		ids, err := s.communities.GetMetaTags(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].MetaTagIDs = ids
	}
	return list, nil
}

func (s *Service) UpdateCommunity(ctx context.Context, id int, name, homeURL string, enabled bool) (Community, error) {
	if name == "" {
		return Community{}, fmt.Errorf("커뮤니티 이름은 필수입니다")
	}
	return s.communities.Update(ctx, id, name, homeURL, enabled)
}

func (s *Service) DeleteCommunity(ctx context.Context, id int) error {
	return s.communities.Delete(ctx, id)
}

func (s *Service) SetMetaTags(ctx context.Context, communityID int, tagIDs []int) error {
	return s.communities.SetMetaTags(ctx, communityID, tagIDs)
}

// --- axes ---

func (s *Service) ListAxes(ctx context.Context) ([]Axis, error) { return s.axes.List(ctx) }

func (s *Service) CreateAxis(ctx context.Context, a Axis) (Axis, error) {
	if a.Key == "" || a.Label == "" {
		return Axis{}, fmt.Errorf("축 key와 label은 필수입니다")
	}
	return s.axes.Create(ctx, a)
}

// --- tags ---

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) { return s.tags.List(ctx) }

func (s *Service) ListTagsByAxis(ctx context.Context, axisID int) ([]Tag, error) {
	return s.tags.ListByAxis(ctx, axisID)
}

func (s *Service) CreateTag(ctx context.Context, t Tag) (Tag, error) {
	if t.AxisID == 0 {
		return Tag{}, fmt.Errorf("축은 필수입니다")
	}
	if t.Name == "" {
		return Tag{}, fmt.Errorf("태그 이름은 필수입니다")
	}
	return s.tags.Create(ctx, t)
}

func (s *Service) UpdateTag(ctx context.Context, id int, name, description string) (Tag, error) {
	if name == "" {
		return Tag{}, fmt.Errorf("태그 이름은 필수입니다")
	}
	return s.tags.Update(ctx, id, name, description)
}

func (s *Service) DeleteTag(ctx context.Context, id int) error { return s.tags.Delete(ctx, id) }
