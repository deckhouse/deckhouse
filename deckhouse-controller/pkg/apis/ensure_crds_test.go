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

package apis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestModuleCRDServingV1Alpha2(t *testing.T) {
	manifest := filepath.Join("..", "..", "crds", moduleCRDFileName)

	onDisk, err := os.ReadFile(manifest)
	require.NoError(t, err)

	rendered, err := moduleCRDServingV1Alpha2(manifest)
	require.NoError(t, err)

	t.Cleanup(func() { _ = os.Remove(rendered) })

	raw, err := os.ReadFile(rendered)
	require.NoError(t, err)

	got := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(raw, &got))

	require.Equal(t,
		map[string][2]bool{"v1alpha1": {true, false}, "v1alpha2": {true, true}},
		versionFlags(t, got),
		"v1alpha1 stays served and gives up the storage to v1alpha2")

	// the flags are the only difference: with the flag off the very same document
	// is applied, so anything else the copy loses is lost on one start out of two
	want := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(onDisk, &want))

	for _, item := range want["spec"].(map[string]any)["versions"].([]any) {
		version := item.(map[string]any)
		version["served"] = true
		version["storage"] = version["name"] == "v1alpha2"
	}

	require.Equal(t, want, got)

	afterwards, err := os.ReadFile(manifest)
	require.NoError(t, err)
	require.Equal(t, onDisk, afterwards, "the manifest on disk keeps the flag-off state")
}

func versionFlags(t *testing.T, crd map[string]any) map[string][2]bool {
	t.Helper()

	versions, ok := crd["spec"].(map[string]any)["versions"].([]any)
	require.True(t, ok)

	flags := make(map[string][2]bool, len(versions))
	for _, item := range versions {
		version, ok := item.(map[string]any)
		require.True(t, ok)

		flags[version["name"].(string)] = [2]bool{version["served"].(bool), version["storage"].(bool)}
	}

	return flags
}
