package proxy

import (
	"context"

	"github.com/docker/distribution"
)

// tagService is uncached version of cachedTagService
type tagService struct {
	remoteTags     distribution.TagService
	authChallenger authChallenger
}

var _ distribution.TagService = tagService{}

// Get attempts to get the most recent digest for the tag by checking the remote
// tag service first and then caching it locally.  If the remote is unavailable
// the local association is returned
func (pt tagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	err := pt.authChallenger.tryEstablishChallenges(ctx)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	desc, err := pt.remoteTags.Get(ctx, tag)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	return desc, nil
}

func (pt tagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	return distribution.ErrUnsupported
}

func (pt tagService) Untag(ctx context.Context, tag string) error {
	return distribution.ErrUnsupported
}

func (pt tagService) All(ctx context.Context) ([]string, error) {
	err := pt.authChallenger.tryEstablishChallenges(ctx)
	if err != nil {
		return []string{}, err
	}

	return pt.remoteTags.All(ctx)
}

func (pt tagService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	return []string{}, distribution.ErrUnsupported
}

// cachedTagService supports local and remote lookup of tags.
type cachedTagService struct {
	tagService

	localTags distribution.TagService
}

var _ distribution.TagService = cachedTagService{}

// Get attempts to get the most recent digest for the tag by checking the remote
// tag service first and then caching it locally.  If the remote is unavailable
// the local association is returned
func (pt cachedTagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	desc, err := pt.tagService.Get(ctx, tag)
	if err == nil {
		err := pt.localTags.Tag(ctx, tag, desc)
		if err != nil {
			return distribution.Descriptor{}, err
		}
		return desc, nil
	}

	desc, err = pt.localTags.Get(ctx, tag)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	return desc, nil
}

func (pt cachedTagService) Untag(ctx context.Context, tag string) error {
	err := pt.localTags.Untag(ctx, tag)
	if err != nil {
		return err
	}
	return nil
}

func (pt cachedTagService) All(ctx context.Context) ([]string, error) {
	tags, err := pt.tagService.All(ctx)
	if err == nil {
		return tags, err
	}

	return pt.localTags.All(ctx)
}
