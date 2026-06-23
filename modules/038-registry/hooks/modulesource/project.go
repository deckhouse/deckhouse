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

package modulesource

import (
	"encoding/json"
	"strings"
)

// MSSnap is the filtered snapshot of a ModuleSource object.
// The FilterFunc in hook.go produces it.
type MSSnap struct {
	// Name is the ModuleSource name.
	Name string `json:"name"`
	// RepoInSpec is spec.registry.repo as currently written in the object.
	// For already-rewritten MSes this points to the local svc; for
	// pre-existing ones it still holds the real upstream repo.
	RepoInSpec string `json:"repoInSpec"`
	// UpstreamJSON is the raw JSON value of the
	// registry.deckhouse.io/upstream annotation, or "" when absent.
	UpstreamJSON string `json:"upstreamJSON,omitempty"`
	// SpecScheme / SpecCA / SpecDockerCfg are the spec.registry fields,
	// used when the annotation is absent (pre-existing / not-yet-rewritten MS).
	SpecScheme    string `json:"specScheme,omitempty"`
	SpecCA        string `json:"specCA,omitempty"`
	SpecDockerCfg string `json:"specDockerCfg,omitempty"`
}

// upstreamRecord mirrors the JSON structure stored in the
// registry.deckhouse.io/upstream annotation (written by the webhook on
// first rewrite).
type upstreamRecord struct {
	Scheme    string `json:"scheme,omitempty"`
	Repo      string `json:"repo"`
	CA        string `json:"ca,omitempty"`
	DockerCfg string `json:"dockerCfg,omitempty"`
}

// Credentials is the credentials block inside an Entry's Upstream.
// JSON tags match the RegistryConfig CRD spec.registries[].upstream.credentials.
type Credentials struct {
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	DockerCfg string `json:"dockerCfg,omitempty"`
}

// Upstream is the real upstream registry descriptor.
// JSON tags match the RegistryConfig CRD spec.registries[].upstream.
type Upstream struct {
	Host        string      `json:"host"`
	Path        string      `json:"path,omitempty"`
	Scheme      string      `json:"scheme,omitempty"`
	CA          string      `json:"ca,omitempty"`
	Credentials Credentials `json:"credentials,omitempty"`
}

// Entry is one element of registry.internal.moduleSourceEntries.
// JSON tags match the RegistryConfig CRD spec.registries[] items.
type Entry struct {
	// Host is the full original repo path used as a virtual path-prefix
	// by the agent (e.g. "nexus.example.com/modules/a").
	Host     string   `json:"host"`
	Upstream Upstream `json:"upstream"`
}

// projectEntries derives the []Entry list and the list of ModuleSource names
// that need a backstop patch (i.e. the annotation is absent, meaning the
// webhook has not yet rewritten the object).
//
// Contract (from the 5a agent):
//   - Entry.Host = realRepo  (the full <orig-host>/<orig-path>)
//   - Entry.Upstream.Host = first path segment of realRepo
//   - Entry.Upstream.Path = remainder
//   - Scheme/CA/Credentials from the real upstream
func projectEntries(snaps []MSSnap) (entries []Entry, toPatch []string) {
	// Start non-nil: registry.internal.moduleSourceEntries is a required array in
	// the values schema, so a fresh cluster with zero ModuleSources must serialize
	// to [] — a nil slice becomes null and fails OpenAPI validation, wedging the
	// registry module's startup hook.
	entries = make([]Entry, 0, len(snaps))
	for _, s := range snaps {
		var realRepo, scheme, ca, dockerCfg string

		if s.UpstreamJSON != "" {
			// Already rewritten: real upstream is encoded in the annotation.
			var ann upstreamRecord
			if err := json.Unmarshal([]byte(s.UpstreamJSON), &ann); err == nil {
				realRepo = ann.Repo
				scheme = ann.Scheme
				ca = ann.CA
				dockerCfg = ann.DockerCfg
			}
		}

		if realRepo == "" {
			// Not yet rewritten: take data directly from spec and queue for backstop.
			realRepo = s.RepoInSpec
			scheme = s.SpecScheme
			ca = s.SpecCA
			dockerCfg = s.SpecDockerCfg
			toPatch = append(toPatch, s.Name)
		}

		host, path, _ := strings.Cut(realRepo, "/")

		entries = append(entries, Entry{
			Host: realRepo,
			Upstream: Upstream{
				Host:   host,
				Path:   path,
				Scheme: scheme,
				CA:     ca,
				Credentials: Credentials{
					DockerCfg: dockerCfg,
				},
			},
		})
	}
	return entries, toPatch
}
