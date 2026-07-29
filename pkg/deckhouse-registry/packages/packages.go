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
	"fmt"

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

// Metadata returns the decoded version.json of the package release image at
// tag. A package release declares only its version, so there is nothing here
// like the rollout controls a Deckhouse release carries.
func (s *VersionService) Metadata(ctx context.Context, tag string) (*release.PackageVersion, error) {
	return s.PackageVersion(ctx, tag)
}

// Definition returns the decoded package.yaml of the release image at tag —
// the package manifest the release publishes. One schema covers both package
// types; use IsModule and IsApplication to tell which this one describes.
//
// Returns release.ErrFileNotFound when the image carries no package.yaml: a
// transitional release may still ship the legacy module.yaml, readable with
// File(ctx, tag, definition.ModuleFile). For the raw bytes, use
// File(ctx, tag, definition.PackageFile).
func (s *VersionService) Definition(ctx context.Context, tag string) (*definition.Package, error) {
	raw, err := s.File(ctx, tag, definition.PackageFile)
	if err != nil {
		return nil, err
	}

	pkg, err := definition.ParsePackage(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Ref(tag), err)
	}

	return pkg, nil
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
