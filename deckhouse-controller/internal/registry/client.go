package registry

import (
	"context"

	"github.com/deckhouse/deckhouse/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type Interface interface {
	WithSegment(segments ...string) Interface
	GetRegistry() string
	GetDigest(ctx context.Context, ref string) (*v1.Hash, error)
	GetManifest(ctx context.Context, ref string) (registry.ManifestResult, error)
	GetImageConfig(ctx context.Context, ref string) (*v1.ConfigFile, error)
	CheckImageExists(ctx context.Context, ref string) error
	GetImage(ctx context.Context, ref string, opts ...registry.ImageGetOption) (registry.Image, error)
	PushImage(ctx context.Context, ref string, img v1.Image, opts ...registry.ImagePushOption) error
	ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error)
	ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error)
}
