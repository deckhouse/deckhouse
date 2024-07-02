/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_utils "system-registry-manager/pkg/utils"
)

func TestCreateStaticPodBundle(t *testing.T) {
	err := generateInputConfigForTest()
	assert.NoError(t, err)

	manifestsSpec := pkg_cfg.NewManifestsSpecForTest()
	params := InputParams{
		Certs:     struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		Manifests: struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		StaticPods: struct {
			UpdateOrCreate       bool
			MasterPeers          []string
			CheckWithMasterPeers bool
		}{
			UpdateOrCreate:       true,
			CheckWithMasterPeers: true,
			MasterPeers:          []string{"123", "321"},
		},
	}

	renderData, err := pkg_cfg.GetDataForManifestRendering(pkg_cfg.NewExtraDataForManifestRendering(params.StaticPods.MasterPeers))
	assert.NoError(t, err)

	for _, staticPod := range manifestsSpec.StaticPods {
		_, err := CreateStaticPodBundle(context.Background(), &staticPod, &renderData)
		assert.NoError(t, err)
	}
}

func TestPrepareStaticPodsBeforeCompare(t *testing.T) {
	tests := []struct {
		name                 string
		content              []byte
		CheckWithMasterPeers bool
		expectedContent      []byte
	}{
		{
			name:                 "CheckWithMasterPeers true",
			CheckWithMasterPeers: true,
			content: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test
  annotations:
    old-annotation-key: old-annotation-value
    certschecksum: abc123
    manifestschecksum: def456
spec:
  containers:
  - name: test
    args:
      - server
      - -master.peers=.......
`),
			expectedContent: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test
  annotations:
    old-annotation-key: old-annotation-value
    certschecksum: ""
    manifestschecksum: ""
    staticpodschecksum: ""
spec:
  containers:
  - name: test
    args:
      - server
      - -master.peers=.......
`),
		},
		{
			name:                 "CheckWithMasterPeers false",
			CheckWithMasterPeers: false,
			content: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test
  annotations:
    old-annotation-key: old-annotation-value
    certschecksum: abc123
    manifestschecksum: def456
spec:
  containers:
  - name: test
    args:
      - server
      - -master.peers=.......
`),
			expectedContent: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test
  annotations:
    old-annotation-key: old-annotation-value
    certschecksum: ""
    manifestschecksum: ""
    staticpodschecksum: ""
spec:
  containers:
  - name: test
    args:
      - server
`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &InputParams{
				StaticPods: struct {
					UpdateOrCreate       bool
					MasterPeers          []string
					CheckWithMasterPeers bool
				}{
					UpdateOrCreate:       true,
					MasterPeers:          []string{"123", "321"},
					CheckWithMasterPeers: tt.CheckWithMasterPeers,
				},
			}
			newContent, err := prepareStaticPodsBeforeCompare(string(tt.content), params)
			if err != nil {
				t.Errorf("Error preparing static pod manifest: %v", err)
			}

			if !pkg_utils.EqualYaml([]byte(newContent), tt.expectedContent) {
				t.Errorf("Expected content:\n%s\n\nActual content:\n%s\n", string(tt.expectedContent), string(newContent))
			}
		})
	}
}

func TestRemoveLineByParams(t *testing.T) {
	tests := []struct {
		name             string
		manifest         []byte
		params           []string
		expectedManifest []byte
	}{
		{
			name:   "Basic test",
			params: []string{"certs-checksum", "manifests-checksum", "static-pods-checksum"},
			manifest: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-static-pod
  annotations:
    old-annotation-key: old-annotation-value
    certs-checksum: abc123
    manifests-checksum: def456
    static-pods-checksum: xyz789
spec:
  # Спецификация пода
`),
			expectedManifest: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-static-pod
  annotations:
    old-annotation-key: old-annotation-value
spec:
  # Спецификация пода
`),
		},
		{
			name:   "Basic test",
			params: []string{"apiVersion", "kind"},
			manifest: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-static-pod
  annotations:
    old-annotation-key: old-annotation-value
    certs-checksum: abc123
    manifests-checksum: def456
    static-pods-checksum: xyz789
spec:
  # Спецификация пода
`),
			expectedManifest: []byte(`
metadata:
  name: my-static-pod
  annotations:
    old-annotation-key: old-annotation-value
    certs-checksum: abc123
    manifests-checksum: def456
    static-pods-checksum: xyz789
spec:
  # Спецификация пода
`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newManifest := removeLineByParams(string(tt.manifest), tt.params)

			expectedManifest := string(tt.expectedManifest)
			if newManifest != expectedManifest {
				t.Errorf("Expected manifest:\n%s\n\nActual manifest:\n%s\n", expectedManifest, newManifest)
			}
		})
	}
}
