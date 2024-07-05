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
			UpdateOrCreate bool
			Options        struct {
				MasterPeers     []string
				IsRaftBootstrap bool
			}
			Check struct {
				WithMasterPeers     bool
				WithIsRaftBootstrap bool
			}
		}{
			UpdateOrCreate: true,
			Options: struct {
				MasterPeers     []string
				IsRaftBootstrap bool
			}{
				MasterPeers:     []string{"123", "321"},
				IsRaftBootstrap: true,
			},
			Check: struct {
				WithMasterPeers     bool
				WithIsRaftBootstrap bool
			}{
				WithMasterPeers:     true,
				WithIsRaftBootstrap: true,
			},
		},
	}

	renderData, err := pkg_cfg.GetDataForManifestRendering(
		pkg_cfg.NewExtraDataForManifestRendering(
			params.StaticPods.Options.MasterPeers,
			params.StaticPods.Options.IsRaftBootstrap,
		),
	)
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
		CheckIsRaftBootstrap bool
		expectedContent      []byte
	}{
		{
			name:                 "CheckWithMasterPeers: true && CheckIsRaftBootstrap: true",
			CheckWithMasterPeers: true,
			CheckIsRaftBootstrap: true,
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
      - -master.raftBootstrap
      - -master.peers="......."
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
      - -master.raftBootstrap
      - -master.peers="......."
`),
		},
		{
			name:                 "CheckWithMasterPeers: true && CheckIsRaftBootstrap: false",
			CheckWithMasterPeers: true,
			CheckIsRaftBootstrap: false,
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
      - -master.raftBootstrap
      - -master.peers="......."
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
      - -master.peers="......."
`),
		},
		{
			name:                 "CheckWithMasterPeers: false && CheckIsRaftBootstrap: true",
			CheckWithMasterPeers: false,
			CheckIsRaftBootstrap: true,
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
      - -master.raftBootstrap
      - -master.peers="......."
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
      - -master.raftBootstrap
`),
		},
		{
			name:                 "CheckWithMasterPeers: false && CheckIsRaftBootstrap: false",
			CheckWithMasterPeers: false,
			CheckIsRaftBootstrap: false,
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
      - -master.raftBootstrap
      - -master.peers="......."
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
					UpdateOrCreate bool
					Options        struct {
						MasterPeers     []string
						IsRaftBootstrap bool
					}
					Check struct {
						WithMasterPeers     bool
						WithIsRaftBootstrap bool
					}
				}{
					UpdateOrCreate: true,
					Options: struct {
						MasterPeers     []string
						IsRaftBootstrap bool
					}{
						MasterPeers:     []string{},
						IsRaftBootstrap: true,
					},
					Check: struct {
						WithMasterPeers     bool
						WithIsRaftBootstrap bool
					}{
						WithMasterPeers:     tt.CheckWithMasterPeers,
						WithIsRaftBootstrap: tt.CheckIsRaftBootstrap,
					},
				},
			}
			newContent, err := prepareStaticPodsBeforeCompare(string(tt.content), params)
			if err != nil {
				t.Errorf("Error preparing static pod manifest: %v", err)
			}

			if !pkg_utils.EqualYaml(tt.expectedContent, []byte(newContent)) {
				t.Errorf("Expected content:\n%s\n\nActual content:\n%s\n", string(tt.expectedContent), newContent)
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

			if !pkg_utils.EqualYaml(tt.expectedManifest, []byte(newManifest)) {
				t.Errorf("Expected manifest:\n%s\n\nActual manifest:\n%s\n", string(tt.expectedManifest), newManifest)
			}
		})
	}
}
