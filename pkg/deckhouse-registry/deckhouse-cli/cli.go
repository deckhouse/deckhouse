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

// Package cli models the deckhouse-cli sub-tree:
//
//	<root>/<edition>/deckhouse-cli:<version>                          CLI image
//	<root>/<edition>/deckhouse-cli/version:<version>                  CLI releases
//	<root>/<edition>/deckhouse-cli/plugins:<plugin>                   plugin catalog
//	<root>/<edition>/deckhouse-cli/plugins/<plugin>:<version>         plugin image
//	<root>/<edition>/deckhouse-cli/plugins/<plugin>/version:<version> plugin releases
//
// The directory is named after the registry segment it owns, deckhouse-cli,
// which is not a valid Go identifier; the package itself is cli. Import it as:
//
//	cli "github.com/deckhouse/deckhouse/pkg/deckhouse-registry/deckhouse-cli"
package cli

import (
	"context"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/internal/cache"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Fixed path segments of the deckhouse-cli sub-tree.
const (
	// Segment is the deckhouse-cli tree under the edition root.
	Segment = "deckhouse-cli"
	// PluginsSegment is the plugin catalog.
	PluginsSegment = "plugins"
	// VersionSegment holds the release images of the CLI and of each plugin.
	VersionSegment = "version"
)

// Service names used in log records.
const (
	ServiceName              = "deckhouse_cli"
	versionServiceName       = "deckhouse_cli_version"
	pluginsServiceName       = "plugins"
	pluginServiceName        = "plugin"
	pluginVersionServiceName = "plugin_version"
)

// Service addresses the deckhouse-cli tree. The service itself is the
// deckhouse-cli image repository.
type Service struct {
	*service.BasicService

	versions *release.Service
	plugins  *PluginCatalog
}

// New wraps a repository that already addresses the deckhouse-cli tree. The
// assembler supplies the path via Sub(ServiceName, Segment); cli fixes the
// version and plugins sub-paths beneath it, its own domain.
func New(svc *service.BasicService) *Service {
	return &Service{
		BasicService: svc,
		versions:     release.New(svc.Sub(versionServiceName, VersionSegment)),
		plugins:      newPluginCatalog(svc.Sub(pluginsServiceName, PluginsSegment)),
	}
}

// Versions returns the deckhouse-cli release repository
// (<root>/<edition>/deckhouse-cli/version).
func (s *Service) Versions() *release.Service {
	return s.versions
}

// Plugins returns the deckhouse-cli plugin catalog
// (<root>/<edition>/deckhouse-cli/plugins).
func (s *Service) Plugins() *PluginCatalog {
	return s.plugins
}

// PluginCatalog is the deckhouse-cli plugin catalog. Like the module and
// package catalogs, its tags are plugin names.
type PluginCatalog struct {
	*service.BasicService

	plugins *cache.Cache[*Plugin]
}

func newPluginCatalog(svc *service.BasicService) *PluginCatalog {
	return &PluginCatalog{
		BasicService: svc,
		plugins:      cache.New[*Plugin](),
	}
}

// List returns the names of the plugins published in this catalog.
func (c *PluginCatalog) List(ctx context.Context) ([]string, error) {
	return c.ListTags(ctx)
}

// Plugin returns the service for a single plugin
// (<root>/<edition>/deckhouse-cli/plugins/<name>). Repeated calls with the same
// name return the same service.
func (c *PluginCatalog) Plugin(name string) *Plugin {
	return c.plugins.Get(name, func() *Plugin {
		svc := c.Named(pluginServiceName, name)

		return &Plugin{
			BasicService: svc,
			versions:     release.New(svc.Sub(pluginVersionServiceName, VersionSegment)),
		}
	})
}

// Plugin addresses one deckhouse-cli plugin.
type Plugin struct {
	*service.BasicService

	versions *release.Service
}

// Versions returns the plugin's release repository
// (<root>/<edition>/deckhouse-cli/plugins/<plugin>/version).
func (p *Plugin) Versions() *release.Service {
	return p.versions
}
