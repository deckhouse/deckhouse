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
	"fmt"

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

// Metadata returns the decoded version.json of the module release image at
// tag. A module release declares only its version, so there is nothing here
// like the rollout controls a Deckhouse release carries.
func (s *ReleaseService) Metadata(ctx context.Context, tag string) (*release.PackageVersion, error) {
	return s.PackageVersion(ctx, tag)
}

// Definition returns the decoded module.yaml of the release image at tag — the
// module manifest the release publishes.
//
// Returns release.ErrFileNotFound when the image carries no manifest, which
// happens on older releases where it must be read from the module image
// instead. For the raw bytes, use File(ctx, tag, definition.ModuleFile).
func (s *ReleaseService) Definition(ctx context.Context, tag string) (*definition.Module, error) {
	raw, err := s.File(ctx, tag, definition.ModuleFile)
	if err != nil {
		return nil, err
	}

	module, err := definition.ParseModule(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Ref(tag), err)
	}

	return module, nil
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
