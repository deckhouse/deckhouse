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

// Package packages models the package sub-tree:
//
//	<root>/<edition>/packages:<package>                  catalog
//	<root>/<edition>/packages/<package>:<version>        package image
//	<root>/<edition>/packages/<package>/version:<...>    package releases
//	<root>/<edition>/packages/<package>/extra/<extra>    auxiliary images
//
// Packages are the v2 abstraction covering both applications and modules: the
// registry contract is the same for either. Structurally a package mirrors a
// module, with two differences — it lives under packages/ instead of modules/,
// and its release images sit under version/ instead of release/.
package packages

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

// Fixed path segments of the package sub-tree.
const (
	// CatalogSegment is the package catalog under the edition root.
	CatalogSegment = "packages"
	// VersionSegment holds a package's release images. It is the package
	// counterpart of the module's release segment.
	VersionSegment = "version"
)

// Service names used in log records.
const (
	CatalogServiceName    = "packages"
	ServiceName           = "package"
	versionServiceName    = "package_version"
	extraServiceName      = "package_extra"
	extraImageServiceName = "package_extra_image"
)

// Catalog is the package catalog at <root>/<edition>/packages, a repository
// whose tags are package names.
type Catalog struct {
	*service.BasicService

	packages *cache.Cache[*Service]
}

// NewCatalog wraps a repository that already addresses the package catalog.
// The assembler supplies the path via Sub(CatalogServiceName, CatalogSegment).
func NewCatalog(svc *service.BasicService) *Catalog {
	return &Catalog{
		BasicService: svc,
		packages:     cache.New[*Service](),
	}
}

// List returns the names of the packages published in this catalog.
func (c *Catalog) List(ctx context.Context) ([]string, error) {
	return c.ListTags(ctx)
}

// Package returns the service for a single package
// (<root>/<edition>/packages/<name>). Repeated calls with the same name return
// the same service.
func (c *Catalog) Package(name string) *Service {
	return c.packages.Get(name, func() *Service {
		return New(c.Named(ServiceName, name))
	})
}

// Service addresses one package.
// VersionService is a package's release repository
// (<root>/<edition>/packages/<package>/version).
//
// It is the package counterpart of module.ReleaseService: same segment role,
// different definition file. A package release ships package.yaml next to
// version.json, and leaves the rollout fields of version.json empty.
type VersionService struct {
	*release.Service
}

// Fetch pulls the package release image at tag once and returns a snapshot that
// serves its version and package.yaml from memory.
func (s *VersionService) Fetch(ctx context.Context, tag string) (*Release, error) {
	raw, err := s.Service.Fetch(ctx, tag)
	if err != nil {
		return nil, err
	}

	return &Release{raw: raw}, nil
}

// Release is a package release image read once. It is the package counterpart
// of module.Release: same version.json, package.yaml instead of module.yaml.
type Release struct {
	raw *release.Release
}

// Metadata returns the decoded version.json. A package release declares only
// its version, without the rollout controls a Deckhouse release carries.
func (r *Release) Metadata() (*release.PackageVersion, error) {
	return r.raw.PackageVersion()
}

// Version returns the version the release declares.
func (r *Release) Version() (string, error) {
	return r.raw.Version()
}

// Definition returns the decoded package.yaml — the package manifest the
// release publishes. One schema covers both package types; use IsModule and
// IsApplication to tell which this one describes.
//
// Returns release.ErrFileNotFound when the image carries no package.yaml: a
// transitional release may still ship the legacy module.yaml, readable with
// File(definition.ModuleFile).
func (r *Release) Definition() (*definition.Package, error) {
	rawYAML, ok := r.raw.File(definition.PackageFile)
	if !ok {
		return nil, fmt.Errorf("%w: %s has no %s", release.ErrFileNotFound, r.raw.Ref(), definition.PackageFile)
	}

	pkg, err := definition.ParsePackage(rawYAML)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.raw.Ref(), err)
	}

	return pkg, nil
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

	versions *VersionService
	extra    *extra.Catalog
}

// New builds the sub-tree of a package whose repository svc addresses.
//
// The package image is an artifact bundle and carries images_digests.json; the
// version and extra sub-repositories are not and do not.
func New(svc *service.BasicService) *Service {
	return &Service{
		Service:  bundle.New(svc, bundle.RootPath),
		versions: &VersionService{Service: release.New(svc.Sub(versionServiceName, VersionSegment))},
		extra:    extra.NewCatalog(svc.Sub(extraServiceName, extra.Segment), extraImageServiceName),
	}
}

// Versions returns the package's release repository
// (<root>/<edition>/packages/<package>/version). It is the package equivalent
// of a module's Releases — same metadata, different segment.
func (s *Service) Versions() *VersionService {
	return s.versions
}

// Extra returns the package's auxiliary image catalog
// (<root>/<edition>/packages/<package>/extra).
func (s *Service) Extra() *extra.Catalog {
	return s.extra
}

// Delete removes one published version of the package from the registry.
//
// It first pulls the bundle at tag to read its images_digests.json — the map of
// every image the package ships to its digest — and deletes those images by
// digest. Only then does it delete the version tag and, last, the bundle tag.
//
// The order is deliberate: the bundle tag is what the image list is read from,
// so removing it last keeps the package recoverable if a run is interrupted — a
// retry re-reads the same list and finishes the job. For that reason Delete
// treats an image or tag that is already gone as success (so it is safe to
// re-run) but stops before the tags when an image cannot be deleted for any
// other reason, leaving the bundle tag as the record to retry from.
func (s *Service) Delete(ctx context.Context, tag string) error {
	b, err := s.Fetch(ctx, tag)
	if err != nil {
		return fmt.Errorf("read package bundle %s: %w", s.Ref(tag), err)
	}

	// Delete every referenced image before the tags that index them. A package
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

	if err := service.IgnoreNotFound(s.Versions().DeleteTag(ctx, tag)); err != nil {
		return fmt.Errorf("delete package version %s: %w", s.Versions().Ref(tag), err)
	}

	if err := service.IgnoreNotFound(s.DeleteTag(ctx, tag)); err != nil {
		return fmt.Errorf("delete package bundle %s: %w", s.Ref(tag), err)
	}

	return nil
}
