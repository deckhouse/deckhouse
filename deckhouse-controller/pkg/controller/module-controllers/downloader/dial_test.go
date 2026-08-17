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

package downloader

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func downloaderFor(repo string) *ModuleDownloader {
	ms := &v1alpha1.ModuleSource{}
	ms.Spec.Registry.Repo = repo
	return &ModuleDownloader{ms: ms}
}

// TestRepositoryDialsTheAgentButLeavesEverythingElseAlone is about the gap between the address a
// ModuleSource records and the address this process can reach.
//
// Once the registry module manages the pull path, the source names the in-cluster registry —
// which nothing here resolves: this pod is on the host network, and the Service behind that name
// exists only on a cluster running the cache. Recording it is still right, because the same
// address is read by whatever renders an image reference; the translation belongs at the point
// of dialling, which is what this covers.
func TestRepositoryDialsTheAgentButLeavesEverythingElseAlone(t *testing.T) {
	tests := []struct {
		name  string
		repo  string
		parts []string
		want  string
	}{{
		name:  "the in-cluster registry, which is what a managed cluster records",
		repo:  registry_const.HostWithPath + "/modules",
		parts: []string{"upmeter"},
		want:  registry_const.ProxyHostWithPath + "/modules/upmeter",
	}, {
		name:  "and the release channel under it",
		repo:  registry_const.HostWithPath + "/modules",
		parts: []string{"upmeter", "release"},
		want:  registry_const.ProxyHostWithPath + "/modules/upmeter/release",
	}, {
		name:  "an ordinary registry, which is what almost every cluster has",
		repo:  "registry.example.com/deckhouse/ee/modules",
		parts: []string{"upmeter", "release"},
		want:  "registry.example.com/deckhouse/ee/modules/upmeter/release",
	}, {
		name:  "a third-party source whose host merely begins like the in-cluster one",
		repo:  registry_const.Host + "-staging/modules",
		parts: []string{"upmeter"},
		want:  registry_const.Host + "-staging/modules/upmeter",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, downloaderFor(tt.repo).repository(tt.parts...))
		})
	}
}
