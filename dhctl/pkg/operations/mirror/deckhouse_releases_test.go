// Copyright 2024 Flant JSC
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
//

package mirror

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestGenerateDeckhouseReleaseManifests(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), t.Name())
	t.Cleanup(func() {
		_ = os.RemoveAll(testDir)
	})

	tests := []struct {
		name             string
		versionsToMirror []semver.Version
		want             string
	}{
		{
			name: "one_release_without_disruptions",
			versionsToMirror: []semver.Version{
				*semver.MustParse("v1.57.3"),
			},
			want: `
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: null
  name: v1.57.3
spec:
  changelog:
    candi:
      fixes:
      - summary: Fix deckhouse containerd start after installing new containerd-deckhouse package.
        pull_request: https://github.com/deckhouse/deckhouse/pull/6329
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.57.3
  requirements:
    containerdOnAllNodes: 'true'
    ingressNginx: '1.1'
    k8s: 1.23.0
    nodesMinimalOSVersionUbuntu: '18.04'
  version: v1.57.3
status:
  approved: false
  message: ""
  transitionTime: "0001-01-01T00:00:00Z"
`,
		},
		{
			name: "one_release_with_disruptions",
			versionsToMirror: []semver.Version{
				*semver.MustParse("v1.56.12"),
			},
			want: `
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: null
  name: v1.56.12
spec:
  changelog:
    candi:
      fixes:
      - summary: Fix deckhouse containerd start after installing new containerd-deckhouse package.
        pull_request: https://github.com/deckhouse/deckhouse/pull/6329
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.56.12
  disruptions:
  - ingressNginx
  requirements:
    containerdOnAllNodes: 'true'
    ingressNginx: '1.1'
    k8s: 1.23.0
    nodesMinimalOSVersionUbuntu: '18.04'
  version: v1.56.12
status:
  approved: false
  message: ""
  transitionTime: "0001-01-01T00:00:00Z"
`,
		},
		{
			name: "many_releases",
			versionsToMirror: []semver.Version{
				*semver.MustParse("v1.56.12"),
				*semver.MustParse("v1.57.5"),
				*semver.MustParse("v1.58.1"),
			},
			want: `---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: null
  name: v1.56.12
spec:
  changelog:
    candi:
      fixes:
      - summary: Fix deckhouse containerd start after installing new containerd-deckhouse package.
        pull_request: https://github.com/deckhouse/deckhouse/pull/6329
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.56.12
  disruptions:
  - ingressNginx
  requirements:
    containerdOnAllNodes: 'true'
    ingressNginx: '1.1'
    k8s: 1.23.0
    nodesMinimalOSVersionUbuntu: '18.04'
  version: v1.56.12
status:
  approved: false
  message: ""
  transitionTime: "0001-01-01T00:00:00Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: null
  name: v1.57.5
spec:
  changelog:
    candi:
      fixes:
      - summary: Fix deckhouse containerd start after installing new containerd-deckhouse package.
        pull_request: https://github.com/deckhouse/deckhouse/pull/6329
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.57.5
  requirements:
    containerdOnAllNodes: 'true'
    ingressNginx: '1.1'
    k8s: 1.23.0
    nodesMinimalOSVersionUbuntu: '18.04'
  version: v1.57.5
status:
  approved: false
  message: ""
  transitionTime: "0001-01-01T00:00:00Z"
---
apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  creationTimestamp: null
  name: v1.58.1
spec:
  changelog:
    candi:
      fixes:
      - summary: Fix deckhouse containerd start after installing new containerd-deckhouse package.
        pull_request: https://github.com/deckhouse/deckhouse/pull/6329
  changelogLink: https://github.com/deckhouse/deckhouse/releases/tag/v1.58.1
  requirements:
    containerdOnAllNodes: 'true'
    ingressNginx: '1.1'
    k8s: 1.23.0
    nodesMinimalOSVersionUbuntu: '18.04'
  version: v1.58.1
status:
  approved: false
  message: ""
  transitionTime: "0001-01-01T00:00:00Z"
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect := require.New(t)
			pathToManifestFile := filepath.Join(testDir, tt.name, "releases.yaml")
			releaseChannelsLayout, err := CreateEmptyImageLayoutAtPath(filepath.Join(testDir, tt.name, "layout"))
			expect.NoError(err)

			for _, version := range tt.versionsToMirror {
				expect.NoError(
					releaseChannelsLayout.AppendImage(
						createDeckhouseReleaseChannelImage(t, version.String()),
						layout.WithAnnotations(map[string]string{
							"org.opencontainers.image.ref.name": "release-channel:v" + version.String(),
						}),
					),
				)
			}

			err = GenerateDeckhouseReleaseManifests(tt.versionsToMirror, pathToManifestFile, releaseChannelsLayout)
			expect.NoError(err)
			expect.FileExists(pathToManifestFile)

			fileContents, err := os.ReadFile(pathToManifestFile)
			expect.NoError(err)
			expect.YAMLEq(tt.want, string(fileContents))
		})
	}
}

func createDeckhouseReleaseChannelImage(t *testing.T, version string) v1.Image {
	t.Helper()

	// FROM scratch
	base := empty.Image
	layers := make([]v1.Layer, 0)

	// COPY ./version.json /version.json
	// COPY ./changelog.yaml /changelog.yaml
	changelog, err := yaml.JSONToYAML([]byte(`{"candi":{"fixes":[{"summary":"Fix deckhouse containerd start after installing new containerd-deckhouse package.","pull_request":"https://github.com/deckhouse/deckhouse/pull/6329"}]}}`))
	require.NoError(t, err)
	versionInfo := fmt.Sprintf(
		`{"disruptions":{"1.56":["ingressNginx"]},"requirements":{"containerdOnAllNodes":"true","ingressNginx":"1.1","k8s":"1.23.0","nodesMinimalOSVersionUbuntu":"18.04"},"version":%q}`,
		"v"+version,
	)
	l, err := crane.Layer(map[string][]byte{
		"version.json":   []byte(versionInfo),
		"changelog.yaml": changelog,
	})
	require.NoError(t, err)
	layers = append(layers, l)

	img, err := mutate.AppendLayers(base, layers...)
	require.NoError(t, err)
	return img
}
