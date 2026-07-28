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

package registry

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	"github.com/deckhouse/deckhouse/pkg/registry/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
)

// newRegistryClient builds a registry client for repo, a full path such as
// "registry.deckhouse.io/deckhouse/fe", from the credentials a registry secret
// or a ModuleSource carries.
func newRegistryClient(repo string, config *utils.RegistryConfig, logger *log.Logger) (registry.Client, error) {
	host, path := splitHostPath(repo)

	opts := []client.Option{
		client.WithUserAgent(config.UserAgent),
		client.WithCA(config.CA),
		client.WithInsecure(strings.EqualFold(config.Scheme, "http")),
		client.WithLogger(logger),
	}

	switch {
	case config.DockerConfig != "":
		// The auth entry is matched against the full repo path, not the host,
		// so a dockercfg scoped to a sub-path still resolves.
		opt, err := client.WithDockercfg(repo, config.DockerConfig)
		if err != nil {
			return nil, fmt.Errorf("build dockercfg auth for %s: %w", repo, err)
		}

		opts = append(opts, opt)

	case config.Login != "":
		opts = append(opts, client.WithLoginPassword(config.Login, config.Password))
	}

	cli := registry.Client(client.New(host, opts...))
	if path != "" {
		cli = cli.WithSegment(strings.Split(path, "/")...)
	}

	return cli, nil
}

// splitHostPath splits "host/a/b" into "host" and "a/b". A bare host yields an
// empty path.
func splitHostPath(repo string) (string, string) {
	trimmed := strings.Trim(repo, "/")

	host, path, found := strings.Cut(trimmed, "/")
	if !found {
		return trimmed, ""
	}

	return host, path
}
