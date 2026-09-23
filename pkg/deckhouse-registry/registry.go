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
// Listing comes in two forms. ListTags and ListRepositories walk the registry's
// cursor to the end, so a result is complete or an error — never a silently
// truncated first page. StreamTags and StreamRepositories hand over one page at
// a time instead, and returning ErrStopStreaming from the visitor ends the walk
// once the caller has seen enough.
//
// Every node exposes Path and Ref, so the same tree also serves as a path
// builder when no registry access is needed.
//
// Beyond reading, a node can also push and delete — for example
// reg.Modules().Module("stronghold").Delete(ctx, "v1.0.1") removes a published
// version, its images and its release tag. The read, push and delete methods
// are grouped into capability interfaces (see package service) so a caller can
// be handed just the subset it needs.
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
//	service       BasicService (the node all embed) and its read/push/delete capabilities
//
// InstallerSegment below is the only segment this package owns, because the
// installer is the only node that is not edition-scoped.
//
// With NoEdition the edition sub-path disappears and the two roots coincide,
// which is how dev roots such as dev-registry.deckhouse.io/sys/deckhouse-oss
// are addressed.
package dhregistry

import (
	"errors"
	"fmt"
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

// ErrNoRootClient is returned by Installer when the tree has no client at the
// non-edition root, because it was built over an edition-scoped one.
var ErrNoRootClient = errors.New("tree has no client at the registry root")

// installerServiceName is used in log records.
const installerServiceName = "installer"

// Registry is the root of the Deckhouse registry structure. It is safe for
// concurrent use.
type Registry struct {
	edition Edition

	// root is the client the tree was built over. It sits at the non-edition
	// root — e.g. registry.deckhouse.io/deckhouse — unless the caller passed a
	// client that already carries the edition, in which case rootPath holds the
	// real root and no client addresses it.
	root registry.Client
	// rootPath is the non-edition root path, always correct: the client's own
	// path, or its parent when the client is edition-scoped.
	rootPath string
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
		edition:  o.edition,
		root:     client,
		rootPath: client.GetRegistry(),
		logger:   logger,
	}

	// A client that already ends with the edition is the edition root itself.
	// Its parent is then known as a path but addressable by no client, since a
	// client can only be scoped further down — never back up.
	editionScoped := o.edition.IsValid() && endsWithSegment(client.GetRegistry(), o.edition.String())

	r.editionRoot = client
	if o.edition.IsValid() && !editionScoped {
		r.editionRoot = client.WithSegment(o.edition.String())
	}

	if editionScoped {
		r.rootPath, _ = SplitEdition(client.GetRegistry())
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
	//
	// An edition-scoped client cannot reach it: scoping that client would
	// address <root>/<edition>/installer, a repository that does not exist.
	// Leaving the node nil is what makes Installer able to say so instead of
	// handing back a service pointing at nothing.
	if !editionScoped {
		r.installer = bundle.New(
			service.NewBasicService(installerServiceName, client.WithSegment(InstallerSegment), logger),
			bundle.CandiImagesDigestsPath,
		)
	}

	return r
}

// NewForPath builds the registry tree from a client whose path may or may not
// already carry an edition. The edition is detected from the last path segment,
// so both registry.deckhouse.io/deckhouse/fe and a custom root such as
// dev-registry.deckhouse.io/sys/deckhouse-oss are handled correctly.
//
// Detection cannot un-scope a client, so when an edition is found the client is
// assumed to be edition-scoped already and is used as the edition root. Root
// then still reports the right path, but nothing addresses it: Installer
// reports ErrNoRootClient. Use New with an explicitly root-scoped client when
// the installer is needed.
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
//
// The path is correct even when the tree was built over an edition-scoped
// client, where it is the parent of that client's path. Reaching it with a
// request is a different matter — see Installer.
func (r *Registry) Root() string {
	return r.rootPath
}

// EditionRoot returns the edition-scoped root path — for example
// "registry.deckhouse.io/deckhouse/fe". Under NoEdition it equals Root.
func (r *Registry) EditionRoot() string {
	return r.editionRoot.GetRegistry()
}

// Client returns the client the tree was built over — the one at the
// non-edition root, unless that client already carried the edition, in which
// case it is the edition root. Compare Root with EditionRoot to tell which.
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
//
// It reports ErrNoRootClient when the tree was built over a client that already
// carries the edition, because a client cannot be scoped back up to its parent.
// Root still gives the path; build the tree with New over a root-scoped client
// when the installer has to be read.
func (r *Registry) Installer() (*bundle.Service, error) {
	if r.installer == nil {
		return nil, fmt.Errorf("%w: %s is scoped to edition %q", ErrNoRootClient, r.editionRoot.GetRegistry(), r.edition)
	}

	return r.installer, nil
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
