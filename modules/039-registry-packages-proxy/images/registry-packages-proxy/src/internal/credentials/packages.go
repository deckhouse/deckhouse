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

package credentials

import (
	"context"
	"encoding/base64"
	encjson "encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

// GetPackagesConfig fetches a PackageRepository CR by name and turns it into a
// registry.PackagesConfig consumable by the proxy. It reproduces the lookup that
// previously lived directly in the proxy (with the controller-runtime client),
// keeping the on-demand semantics on every request.
func (w *Watcher) GetPackagesConfig(packageRepositoryName string) (*registry.PackagesConfig, error) {
	if w.packageRepositoryClient == nil {
		return nil, fmt.Errorf("package repository client is not configured")
	}

	var pr PackageRepository
	if err := w.packageRepositoryClient.Get(context.Background(), types.NamespacedName{Name: packageRepositoryName}, &pr); err != nil {
		return nil, fmt.Errorf("get package repository %q: %w", packageRepositoryName, err)
	}

	auth, err := resolveRegistryAuth(pr.Spec.Registry)
	if err != nil {
		return nil, fmt.Errorf("resolve registry auth for package repository %q: %w", packageRepositoryName, err)
	}

	return &registry.PackagesConfig{
		Repository: pr.Spec.Registry.Repo,
		Scheme:     pr.Spec.Registry.Scheme,
		CA:         pr.Spec.Registry.CA,
		Auth:       auth,
	}, nil
}

// resolveRegistryAuth converts the credentials carried on a
// PackageRepositorySpecRegistry into the base64("login:password") shape that
// registry.PackagesConfig.Auth (and the docker config "auth" field) expect.
//
// Precedence matches the upstream PackageRepository controller (see
// deckhouse-controller/internal/registry.Service.buildRegistryClient):
//
//  1. spec.registry.login  - takes priority, wins over dockerCfg.
//  2. spec.registry.dockerCfg - parsed as a docker config JSON and matched
//     against spec.registry.repo by URL host.
//  3. neither              - anonymous registry; return empty string.
//
// We do NOT treat the absence of credentials as an error: many package
// repositories are public.
func resolveRegistryAuth(spec PackageRepositorySpecRegistry) (string, error) {
	if spec.Login != "" {
		return encodeBasicAuth(spec.Login, spec.Password), nil
	}
	if spec.DockerCFG == "" {
		return "", nil
	}
	ac, err := readAuthFromDockerCfg(spec.Repo, spec.DockerCFG)
	if err != nil {
		return "", fmt.Errorf("read auth from docker cfg: %w", err)
	}
	// dockerCfg entries usually carry an already-base64-encoded "auth"
	// field; fall back to encoding from username/password if only those
	// are present (some registries / CI tools write the config that way).
	if ac.Auth != "" {
		return ac.Auth, nil
	}
	if ac.Username != "" {
		return encodeBasicAuth(ac.Username, ac.Password), nil
	}
	return "", nil
}

// encodeBasicAuth returns base64("user:password") which is exactly what the
// "auth" field of a docker config JSON entry contains and what
// registry.ClientConfig.Auth is plumbed into authn.AuthConfig.Auth for the
// Basic auth header.
func encodeBasicAuth(user, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
}

// readAuthFromDockerCfg locates the matching auth entry inside a base64-encoded
// docker config JSON, comparing entries by URL host so that a repository like
// "registry.test/path" still matches an "registry.test" key.
//
// The CR contract is that spec.registry.dockerCfg is base64. We do NOT silently
// fall back to interpreting it as raw JSON: that just masks misconfiguration.
func readAuthFromDockerCfg(repo, dockerCfgBase64 string) (authn.AuthConfig, error) {
	r, err := parseRegistryURL(repo)
	if err != nil {
		return authn.AuthConfig{}, fmt.Errorf("parse repo: %w", err)
	}

	dockerCfg, err := base64.StdEncoding.DecodeString(dockerCfgBase64)
	if err != nil {
		return authn.AuthConfig{}, fmt.Errorf("decode dockerCfg as base64: %w", err)
	}
	auths := gjson.Get(string(dockerCfg), "auths").Map()
	authConfig := authn.AuthConfig{}

	for repoName, repoAuth := range auths {
		repoNameURL, err := parseRegistryURL(repoName)
		if err != nil {
			return authn.AuthConfig{}, fmt.Errorf("parse repo name: %w", err)
		}

		if repoNameURL.Host == r.Host {
			if err := encjson.Unmarshal([]byte(repoAuth.Raw), &authConfig); err != nil {
				return authn.AuthConfig{}, fmt.Errorf("unmarshal json: %w", err)
			}
			return authConfig, nil
		}
	}

	return authn.AuthConfig{}, fmt.Errorf("%q credentials not found in the dockerCfg", repo)
}

// parseRegistryURL parses a registry URL that may or may not carry a scheme.
// Without a scheme, url.Parse stuffs the host into Path, so prefix "//" forces
// authority-form parsing and keeps Host populated.
func parseRegistryURL(rawURL string) (*url.URL, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return url.ParseRequestURI(rawURL)
	}
	return url.Parse("//" + rawURL)
}
