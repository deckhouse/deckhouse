// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package module models the module sub-tree:
//
//	<root>/<edition>/modules:<module>                   catalog
//	<root>/<edition>/modules/<module>:<version>         module image
//	<root>/<edition>/modules/<module>/release:<...>     module releases
//	<root>/<edition>/modules/<module>/extra/<extra>     auxiliary images
package module

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/extra"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/internal/cache"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Fixed path segments of the module sub-tree.
const (
	// CatalogSegment is the module catalog under the edition root.
	CatalogSegment = "modules"
	// ReleaseSegment holds a module's release images.
	ReleaseSegment = "release"
)

// Service names used in log records.
const (
	CatalogServiceName    = "modules"
	ServiceName           = "module"
	releaseServiceName    = "module_release"
	extraServiceName      = "module_extra"
	extraImageServiceName = "module_extra_image"
)

// Catalog is the module catalog at <root>/<edition>/modules.
//
// The catalog is itself a repository whose tags are module names, each pointing
// at a scratch image. Listing its tags therefore enumerates the modules the
// edition publishes.
type Catalog struct {
	*service.BasicService

	modules *cache.Cache[*Service]
}

// NewCatalog wraps a repository that already addresses a module catalog — its
// tags are module names, with <module> and <module>/release beneath it.
//
// It does not scope a segment: whoever assembles the tree supplies the catalog
// path. From an edition root that is
// editionRoot.Sub(CatalogServiceName, CatalogSegment); a ModuleSource passes
// its spec.registry.repo, which is the catalog itself.
func NewCatalog(svc *service.BasicService) *Catalog {
	return &Catalog{
		BasicService: svc,
		modules:      cache.New[*Service](),
	}
}

// List returns the names of the modules published in this catalog.
func (c *Catalog) List(ctx context.Context) ([]string, error) {
	return c.ListTags(ctx)
}

// Module returns the service for a single module
// (<root>/<edition>/modules/<name>). Repeated calls with the same name return
// the same service.
func (c *Catalog) Module(name string) *Service {
	return c.modules.Get(name, func() *Service {
		return New(c.Named(ServiceName, name))
	})
}

// Service addresses one module.
// ReleaseService is a module's release repository
// (<root>/<edition>/modules/<module>/release).
//
// It differs from a Deckhouse release in what the image carries: a module
// release ships module.yaml next to version.json, and leaves the rollout
// fields of version.json empty.
type ReleaseService struct {
	*release.Service
}

// Fetch pulls the module release image at tag once and returns a snapshot that
// serves its version and module.yaml from memory.
func (s *ReleaseService) Fetch(ctx context.Context, tag string) (*Release, error) {
	raw, err := s.Service.Fetch(ctx, tag)
	if err != nil {
		return nil, err
	}

	return &Release{raw: raw}, nil
}

// Release is a module release image read once.
type Release struct {
	raw *release.Release
}

// Metadata returns the decoded version.json. A module release declares only
// its version, without the rollout controls a Deckhouse release carries.
func (r *Release) Metadata() (*release.PackageVersion, error) {
	return r.raw.PackageVersion()
}

// Version returns the version the release declares.
func (r *Release) Version() (string, error) {
	return r.raw.Version()
}

// Definition returns the decoded module.yaml — the module manifest the release
// publishes.
//
// Returns release.ErrFileNotFound when the image carries no manifest, which
// happens on older releases where it must be read from the module image
// instead. For the raw bytes, use File(definition.ModuleFile).
func (r *Release) Definition() (*definition.Module, error) {
	rawYAML, ok := r.raw.File(definition.ModuleFile)
	if !ok {
		return nil, fmt.Errorf("%w: %s has no %s", release.ErrFileNotFound, r.raw.Ref(), definition.ModuleFile)
	}

	module, err := definition.ParseModule(rawYAML)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.raw.Ref(), err)
	}

	return module, nil
}

// Changelog returns the decoded changelog, or release.ErrFileNotFound when the
// image carries none.
func (r *Release) Changelog() (map[string]any, error) {
	return r.raw.Changelog()
}

// File returns a raw file from the release image and whether it was present.
func (r *Release) File(name string) ([]byte, bool) {
	return r.raw.File(name)
}

type Service struct {
	*bundle.Service

	releases *ReleaseService
	extra    *extra.Catalog
}

// New builds the sub-tree of a module whose repository svc addresses.
//
// The module image is an artifact bundle and carries images_digests.json; the
// release and extra sub-repositories are not and do not.
func New(svc *service.BasicService) *Service {
	return &Service{
		Service:  bundle.New(svc, bundle.RootPath),
		releases: &ReleaseService{Service: release.New(svc.Sub(releaseServiceName, ReleaseSegment))},
		extra:    extra.NewCatalog(svc.Sub(extraServiceName, extra.Segment), extraImageServiceName),
	}
}

// Releases returns the module's release repository, whose tags are channel
// names and versions.
func (s *Service) Releases() *ReleaseService {
	return s.releases
}

// Extra returns the module's auxiliary image catalog
// (<root>/<edition>/modules/<module>/extra).
func (s *Service) Extra() *extra.Catalog {
	return s.extra
}

// Delete removes one published version of the module from the registry.
//
// It first pulls the bundle at tag to read its images_digests.json — the map of
// every image the module ships to its digest — and deletes those images by
// digest. Only then does it delete the release tag and, last, the bundle tag.
//
// The order is deliberate: the bundle tag is what the image list is read from,
// so removing it last keeps the module recoverable if a run is interrupted — a
// retry re-reads the same list and finishes the job. For that reason Delete
// treats an image or tag that is already gone as success (so it is safe to
// re-run) but stops before the tags when an image cannot be deleted for any
// other reason, leaving the bundle tag as the record to retry from.
func (s *Service) Delete(ctx context.Context, tag string) error {
	b, err := s.Fetch(ctx, tag)
	if err != nil {
		return fmt.Errorf("read module bundle %s: %w", s.Ref(tag), err)
	}

	// Delete every referenced image before the tags that index them. A module
	// bundle carries a flat name-to-digest map, so Digests().Images is it.
	var errs []error

	for image, digest := range b.Digests().Images {
		hash, err := v1.NewHash(digest)
		if err != nil {
			errs = append(errs, fmt.Errorf("image %q has an invalid digest %q: %w", image, digest, err))

			continue
		}

		if err := service.IgnoreNotFound(s.DeleteByDigest(ctx, hash)); err != nil {
			errs = append(errs, fmt.Errorf("delete image %q: %w", image, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("delete images of %s: %w", s.Ref(tag), errors.Join(errs...))
	}

	if err := service.IgnoreNotFound(s.Releases().DeleteTag(ctx, tag)); err != nil {
		return fmt.Errorf("delete module release %s: %w", s.Releases().Ref(tag), err)
	}

	if err := service.IgnoreNotFound(s.DeleteTag(ctx, tag)); err != nil {
		return fmt.Errorf("delete module bundle %s: %w", s.Ref(tag), err)
	}

	return nil
}
