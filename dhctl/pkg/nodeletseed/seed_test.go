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

package nodeletseed

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// The checkout's candi directory, three levels above this package.
const candiDir = "../../../candi"

// candiWith is a candi directory holding this platform document alone.
func candiWith(t *testing.T, document string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(templatePath)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, templatePath), []byte(document), 0o644))
	return dir
}

func images(packages map[string]any) map[string]any {
	return map[string]any{"registrypackages": packages}
}

var installerProxy = []string{"http://127.0.0.1:5444"}

const twoPackages = `apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
spec:
  systemType: Mutable
  featureGates:
    extensions: true
  extensions:
  - name: nodelet
    digest: {{ .images.registrypackages.nodelet }}
  - name: jq
    digest: {{ .images.registrypackages.jq }}
`

func TestFirstMasterRendersThePlatformDocumentForTheInstallerProxy(t *testing.T) {
	doc, err := FirstMaster(t.Context(), candiWith(t, twoPackages),
		images(map[string]any{"nodelet": "sha256:agent", "jq": "sha256:jq"}), installerProxy)
	require.NoError(t, err)

	require.Equal(t, &Document{
		APIVersion: "internal.deckhouse.io/v1alpha1",
		Kind:       "NodeConfig",
		Spec: Spec{
			SystemType:                          "Mutable",
			RegistryPackagesProxyEndpoints:      []string{"http://127.0.0.1:5444"},
			RegistryPackagesProxyAccessTokenB64: base64.StdEncoding.EncodeToString([]byte("passthrough")),
			FeatureGates:                        map[string]bool{"extensions": true},
			Extensions: []Extension{
				{Name: "nodelet", Digest: "sha256:agent", RequestedBy: "node-manager"},
				{Name: "jq", Digest: "sha256:jq", RequestedBy: "node-manager"},
			},
		},
	}, doc)
}

// A package the release did not build would be waited for on the first master until
// bashible gives up: bootstrap stops before the node exists.
func TestFirstMasterRefusesAReleaseMissingAPackage(t *testing.T) {
	_, err := FirstMaster(t.Context(), candiWith(t, twoPackages), images(map[string]any{"nodelet": "sha256:agent"}), installerProxy)

	require.ErrorContains(t, err, "jq")
}

// A field the agent does not know would be refused on the node, after the machine is built.
func TestFirstMasterRefusesAnUnknownField(t *testing.T) {
	_, err := FirstMaster(t.Context(), candiWith(t, twoPackages+"  kubelet: {}\n"),
		images(map[string]any{"nodelet": "sha256:agent", "jq": "sha256:jq"}), installerProxy)

	require.ErrorContains(t, err, "kubelet")
}

// Every edition ships the agent: a release without it is broken, not one bashible serves alone.
func TestFirstMasterRefusesAReleaseWithoutTheAgent(t *testing.T) {
	_, err := FirstMaster(t.Context(), candiWith(t, twoPackages), images(map[string]any{"jq": "sha256:jq"}), installerProxy)

	require.ErrorContains(t, err, `map has no entry for key "nodelet"`)
}

// Without a packages proxy the agent cannot install a single package on the first master.
func TestFirstMasterRefusesASeedWithoutProxyEndpoints(t *testing.T) {
	_, err := FirstMaster(t.Context(), candiWith(t, twoPackages),
		images(map[string]any{"nodelet": "sha256:agent", "jq": "sha256:jq"}), nil)

	require.ErrorContains(t, err, "no packages proxy endpoints")
}

// The document the release ships renders, and names every package once.
func TestTheShippedPlatformDocumentRenders(t *testing.T) {
	text, err := os.ReadFile(filepath.Join(candiDir, templatePath))
	require.NoError(t, err)
	packages := map[string]any{}
	for _, key := range imageKeys(string(text)) {
		packages[key] = "sha256:" + key
	}

	doc, err := FirstMaster(t.Context(), candiDir, images(packages), installerProxy)
	require.NoError(t, err)

	require.Equal(t, "Mutable", doc.Spec.SystemType)
	require.Equal(t, "nodelet", doc.Spec.Extensions[0].Name)
	seen := map[string]bool{}
	for _, ext := range doc.Spec.Extensions {
		require.False(t, seen[ext.Name], "%s is listed twice", ext.Name)
		seen[ext.Name] = true
	}
}

// imageKeys are the registrypackages keys a platform document asks for.
func imageKeys(document string) []string {
	var keys []string
	for _, match := range regexp.MustCompile(`\.images\.registrypackages\.(\w+)`).FindAllStringSubmatch(document, -1) {
		keys = append(keys, match[1])
	}
	return keys
}
