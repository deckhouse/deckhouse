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

// Package dhregistry models the artifact registry structure of the Deckhouse
// ecosystem.
//
// It turns registry paths into a typed tree so callers never build them by
// hand. A root path (registry.deckhouse.io/deckhouse) and an edition (fe) fix
// the top of the tree; everything below it is reached by navigating:
//
//	reg := dhregistry.New(client, dhregistry.WithEdition(dhregistry.FEEdition))
//
//	rel, err := reg.Modules().Module("stronghold").Releases().Fetch(ctx, "alpha")
//	rel.Version()      // resolve the channel to a version
//	rel.Definition()   // decoded module.yaml, from the same pull
//	reg.Security().Image("trivy-db").GetImage(ctx, "2")
//
// A release or bundle image is pulled once by Fetch into a snapshot, which then
// serves every field from memory — reading many fields costs one pull.
//
// Every node exposes Path and Ref, so the same tree also serves as a path
// builder when no registry access is needed.
//
// # Layout
//
// This package holds the vocabulary shared across the whole library — Edition,
// Channel and the sentinel errors — and assembles the tree. Each sub-tree lives
// in its own package, which declares the path segments it owns:
//
//	deckhouse     <root>/<edition>, /release-channel, /install, /install-standalone
//	module        <root>/<edition>/modules[/<module>[/release|/extra/<extra>]]
//	packages      <root>/<edition>/packages[/<package>[/version|/extra/<extra>]]
//	security      <root>/<edition>/security/<name>
//	deckhouse-cli <root>/<edition>/deckhouse-cli[/version|/plugins/<plugin>[/version]]
//	extra         the /extra/<name> catalog shared by module and packages
//	bundle        the repositories holding full images, and their digests
//	release       the release-image reader, and the release channel vocabulary
//	definition    the module.yaml and package.yaml mappings
//	service       BasicService, the single-repository node all of them embed
//
// InstallerSegment below is the only segment this package owns, because the
// installer is the only node that is not edition-scoped.
//
// With NoEdition the edition sub-path disappears and the two roots coincide,
// which is how dev roots such as dev-registry.deckhouse.io/sys/deckhouse-oss
// are addressed.
package dhregistry

import (
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/deckhouse"
	cli "github.com/deckhouse/deckhouse/pkg/deckhouse-registry/deckhouse-cli"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/module"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/packages"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/security"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// InstallerSegment is the edition-independent installer: <root>/installer.
const InstallerSegment = "installer"

// installerServiceName is used in log records.
const installerServiceName = "installer"

// Registry is the root of the Deckhouse registry structure. It is safe for
// concurrent use.
type Registry struct {
	edition Edition

	// root is the client at the non-edition root, e.g.
	// registry.deckhouse.io/deckhouse.
	root registry.Client
	// editionRoot is root scoped to the edition sub-path, or root itself under
	// NoEdition.
	editionRoot registry.Client

	deckhouse *deckhouse.Service
	modules   *module.Catalog
	packages  *packages.Catalog
	security  *security.Catalog
	cli       *cli.Service
	installer *bundle.Service

	logger *log.Logger
}

// Option configures a Registry.
type Option func(*options)

type options struct {
	edition Edition
	logger  *log.Logger
}

// WithEdition scopes the edition-dependent parts of the tree to edition. The
// default is NoEdition, which addresses them directly under the root.
func WithEdition(e Edition) Option {
	return func(o *options) {
		o.edition = e
	}
}

// WithLogger sets the logger the whole tree writes debug records to.
func WithLogger(logger *log.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// New builds the registry tree over a client pointing at the registry root —
// the path above the edition, such as registry.deckhouse.io/deckhouse.
//
// When the client already ends with the configured edition segment the edition
// is not appended twice, so a client built as client.New(host).WithSegment(
// "deckhouse", "fe") works with WithEdition(FEEdition) as well.
func New(client registry.Client, opts ...Option) *Registry {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	logger := o.logger
	if logger == nil {
		logger = log.NewLogger().Named("deckhouse-registry")
	}

	r := &Registry{
		edition: o.edition,
		root:    client,
		logger:  logger,
	}

	r.editionRoot = client
	if o.edition.IsValid() && !endsWithSegment(client.GetRegistry(), o.edition.String()) {
		r.editionRoot = client.WithSegment(o.edition.String())
	}

	// The edition root doubles as the Deckhouse image repository, so it is the
	// parent every edition-scoped sub-tree is built from.
	editionRoot := service.NewBasicService(deckhouse.ServiceName, r.editionRoot, logger)

	r.deckhouse = deckhouse.New(editionRoot)
	r.modules = module.NewCatalog(editionRoot.Sub(module.CatalogServiceName, module.CatalogSegment))
	r.packages = packages.NewCatalog(editionRoot.Sub(packages.CatalogServiceName, packages.CatalogSegment))
	r.security = security.NewCatalog(editionRoot.Sub(security.CatalogServiceName, security.Segment))
	r.cli = cli.New(editionRoot.Sub(cli.ServiceName, cli.Segment))

	// The installer is published once for all editions, so it hangs off the
	// non-edition root. It is a bundle like the edition-scoped installers, and
	// keeps its digests in the same place.
	r.installer = bundle.New(
		service.NewBasicService(installerServiceName, client.WithSegment(InstallerSegment), logger),
		bundle.CandiImagesDigestsPath,
	)

	return r
}

// NewForPath builds the registry tree from a client whose path may or may not
// already carry an edition. The edition is detected from the last path segment,
// so both registry.deckhouse.io/deckhouse/fe and a custom root such as
// dev-registry.deckhouse.io/sys/deckhouse-oss are handled correctly.
//
// Detection cannot un-scope a client, so when an edition is found the client is
// assumed to be edition-scoped already and is used as the edition root. The
// non-edition root is then only usable as a path (Root), not for requests: use
// New with an explicitly root-scoped client when the installer is needed.
func NewForPath(client registry.Client, opts ...Option) *Registry {
	_, e := SplitEdition(client.GetRegistry())

	return New(client, append([]Option{WithEdition(e)}, opts...)...)
}

// Edition returns the edition the tree is scoped to.
func (r *Registry) Edition() Edition {
	return r.edition
}

// Root returns the registry root path without the edition — for example
// "registry.deckhouse.io/deckhouse". Use it for the edition-independent
// installer; everything else lives under EditionRoot.
func (r *Registry) Root() string {
	return r.root.GetRegistry()
}

// EditionRoot returns the edition-scoped root path — for example
// "registry.deckhouse.io/deckhouse/fe". Under NoEdition it equals Root.
func (r *Registry) EditionRoot() string {
	return r.editionRoot.GetRegistry()
}

// Client returns the client at the non-edition root.
func (r *Registry) Client() registry.Client {
	return r.root
}

// Deckhouse returns the Deckhouse platform services: the platform image itself,
// its release channels, and both installers.
func (r *Registry) Deckhouse() *deckhouse.Service {
	return r.deckhouse
}

// Modules returns the module catalog.
func (r *Registry) Modules() *module.Catalog {
	return r.modules
}

// Packages returns the package catalog — the v2 abstraction shared by
// applications and modules.
func (r *Registry) Packages() *packages.Catalog {
	return r.packages
}

// Security returns the security image catalog.
func (r *Registry) Security() *security.Catalog {
	return r.security
}

// Installer returns the edition-independent installer repository
// (<root>/installer). The edition-scoped installers are on Deckhouse.
func (r *Registry) Installer() *bundle.Service {
	return r.installer
}

// CLI returns the deckhouse-cli tree, including its plugin catalog.
func (r *Registry) CLI() *cli.Service {
	return r.cli
}

// endsWithSegment reports whether path's last "/"-separated segment is segment.
func endsWithSegment(path, segment string) bool {
	trimmed := strings.TrimSuffix(path, "/")

	return trimmed == segment || strings.HasSuffix(trimmed, "/"+segment)
}
