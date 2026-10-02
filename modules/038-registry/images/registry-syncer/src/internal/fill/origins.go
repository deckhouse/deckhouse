/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fill

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// ResolveOrigins says, for a fill from the upstream, where each module has to be read from.
//
// Every module the cluster keeps is in the set, whichever ModuleSource it came from: a module the
// operator installed is one the cluster needs, and after the move to air-gap the store is the only
// place left to get it from. Where the store keeps it is decided by its source alone — see StorePath.
// What is decided here is only where the fill reads it:
//
//   - the platform's own modules, from the upstream, as the platform itself;
//   - a source on the upstream's own `modules` repository — or one whose registry could not be read —
//     from the upstream too, with the upstream's credentials;
//   - a source already pointed at its path in the store, from the store itself: nothing is fetched
//     from outside, and the copy is a check that it is there;
//   - a source on the in-cluster address outside the store's paths — the Flant source on FE, rendered
//     from the in-cluster address — from the upstream at the same path, with the upstream's credentials;
//   - any other source, from its own registry, with its own credentials and authority.
//
// For the cluster to pull a source's modules from the store once the upstream is gone, the source is
// repointed at its path there — `<store>/module-sources/<name>` — before going air-gap.
func ResolveOrigins(modules []ModuleRef, upstream, local Registry) ([]ModuleRef, error) {
	upstreamModules := normalizedRepository(upstream.Address + "/" + upstream.Repository + "/" + modulesRepository)
	storeRoot := normalizedRepository(constant.Host + constant.Path)

	fromUpstream := upstream
	fromUpstream.Repository = strings.Trim(upstream.Repository+"/"+modulesRepository, "/")

	origins := map[string]*Registry{}
	resolved := make([]ModuleRef, 0, len(modules))
	for _, module := range modules {
		module.Origin = nil
		if module.FromPlatform() {
			resolved = append(resolved, module)
			continue
		}

		repo := normalizedRepository(module.Source.Repository)
		if upstreamSibling(repo, storeRoot, upstream) {
			// The in-cluster address outside the store's own paths is the upstream's registry at the same
			// path — which is how the node agent serves it, and how the platform renders the Flant
			// source on FE: `<address>/flant/modules`, with the address the in-cluster one once the
			// cluster pulls through the agent. Dialled as written, it is the store under the module's
			// own authority, which this client does not trust; and the store does not hold it anyway.
			sibling := upstream
			sibling.Repository = strings.TrimPrefix(repo, inClusterHost()+"/")
			module.Origin = &sibling
			resolved = append(resolved, module)
			continue
		}
		switch repo {
		case "", upstreamModules, storeRoot + "/" + modulesRepository:
			// On the upstream's registry, or pointed at the platform's own path in the store, which
			// is the upstream's through the cache: read where the upstream keeps it.
			module.Origin = &fromUpstream
		case storeRoot + "/" + SourceStorePath(module.Source.Name):
			store := local
			store.Repository = strings.Trim(local.Repository, "/") + "/" + SourceStorePath(module.Source.Name)
			module.Origin = &store
		default:
			origin, found := origins[module.Source.Name]
			if !found {
				built, err := sourceRegistry(module.Source)
				if err != nil {
					return nil, fmt.Errorf("module %s from source %s: %w", module.Name, module.Source.Name, err)
				}
				origin = &built
				origins[module.Source.Name] = origin
			}
			module.Origin = origin
		}
		resolved = append(resolved, module)
	}
	return resolved, nil
}

// inClusterHost is the in-cluster registry's host as normalizedRepository spells it.
func inClusterHost() string { return strings.ToLower(constant.Host) }

// upstreamSibling reports whether repo names the in-cluster registry outside the store's paths, with
// an upstream to read it from instead.
func upstreamSibling(repo, storeRoot string, upstream Registry) bool {
	if upstream.Address == "" || !strings.HasPrefix(repo, inClusterHost()+"/") {
		return false
	}
	return repo != storeRoot && !strings.HasPrefix(repo, storeRoot+"/")
}

// Route sends one repository the fill reads to where the store keeps it.
type Route struct {
	// From is the registry and the exact repository read, with the options to read it.
	From Registry

	// Into is the repository in the destination, in full.
	Into string
}

// Routes are where the fill puts what it reads from the modules' origins: each module's package and
// images under its StorePath, and each source's catalogue under its SourceStorePath.
//
// By exact repository rather than by prefix: a source read from the upstream shares the upstream's
// `modules` prefix with the platform's own modules, and a prefix would carry those along with it.
func Routes(modules []ModuleRef, destination Registry) []Route {
	into := func(path string) string {
		if root := strings.Trim(destination.Repository, "/"); root != "" {
			return root + "/" + path
		}
		return path
	}

	catalogued := map[string]bool{}
	var routes []Route
	for _, module := range modules {
		if module.Origin == nil {
			continue
		}
		from := *module.Origin
		from.Repository = strings.Trim(module.Origin.Repository, "/") + "/" + module.Name
		routes = append(routes, Route{From: from, Into: into(module.StorePath())})

		if !catalogued[module.Source.Name] {
			catalogued[module.Source.Name] = true
			catalogue := *module.Origin
			catalogue.Repository = strings.Trim(module.Origin.Repository, "/")
			routes = append(routes, Route{From: catalogue, Into: into(SourceStorePath(module.Source.Name))})
		}
	}
	return routes
}

// sourceRegistry is a ModuleSource's registry as the fill talks to it.
func sourceRegistry(source ModuleSource) (Registry, error) {
	repo := strings.TrimRight(strings.TrimSpace(source.Repository), "/")
	lower := strings.ToLower(repo)
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(lower, scheme) {
			repo = repo[len(scheme):]
			break
		}
	}
	host, path, _ := strings.Cut(repo, "/")
	if host == "" {
		return Registry{}, fmt.Errorf("the repository %q names no registry", source.Repository)
	}

	var username, password string
	if source.DockerCfg != "" {
		raw, err := base64.StdEncoding.DecodeString(source.DockerCfg)
		if err != nil {
			// Said with its likeliest cause: the field is sensitive data, and a reader not allowed
			// `modulesources/sensitive` is handed a placeholder that is not base64 at all.
			return Registry{}, fmt.Errorf("decoding its dockerCfg (a reader without modulesources/sensitive "+
				"is handed a placeholder): %w", err)
		}
		if username, password, err = dockerCfgCredentials(raw, host); err != nil {
			return Registry{}, fmt.Errorf("reading its credentials for %s: %w", host, err)
		}
	}

	options, err := RegistryOptions(source.CA, username, password)
	if err != nil {
		return Registry{}, err
	}

	return Registry{
		Address:    host,
		Repository: path,
		Insecure:   strings.EqualFold(source.Scheme, "http"),
		Options:    options,
	}, nil
}

// dockerCfgCredentials reads the credentials a Docker config holds for a registry host.
//
// Here rather than from go_lib's helpers, which would bring their validation library into this image
// for the sake of one lookup. The entry is found by host, with or without a scheme, and may carry
// either the combined `auth` or the two fields.
func dockerCfgCredentials(raw []byte, host string) (string, string, error) {
	var config struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", "", fmt.Errorf("decoding the Docker config: %w", err)
	}

	for key, entry := range config.Auths {
		if normalizedRepository(key) != strings.ToLower(host) {
			continue
		}
		if entry.Auth == "" {
			return entry.Username, entry.Password, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return "", "", fmt.Errorf("decoding the auth for %s: %w", host, err)
		}
		username, password, found := strings.Cut(string(decoded), ":")
		if !found {
			return "", "", fmt.Errorf("the auth for %s is not user:password", host)
		}
		return username, password, nil
	}
	// No entry: an anonymous source, which a public registry is.
	return "", "", nil
}

// normalizedRepository is a repository comparable with another: no scheme, no trailing slash, the
// host in lower case. The path is left as it is — registries treat it as case-sensitive.
func normalizedRepository(repo string) string {
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(repo), scheme) {
			repo = strings.Trim(repo[len(scheme):], "/")
			break
		}
	}
	host, path, found := strings.Cut(repo, "/")
	if !found {
		return strings.ToLower(repo)
	}
	return strings.ToLower(host) + "/" + strings.Trim(strings.ReplaceAll(path, "//", "/"), "/")
}
