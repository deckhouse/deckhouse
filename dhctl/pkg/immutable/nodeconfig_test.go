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

package immutable

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
)

// maxPods must match what bashible computes for every other node
// (candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl) — the scheduler
// believes it — including the prefixes outside the ladder's own range.
func TestNodeConfigMaxPodsFollowsThePodSubnet(t *testing.T) {
	tests := []struct {
		prefix  string
		expect  int
		comment string
	}{
		{prefix: "26", expect: 120},
		{prefix: "24", expect: 120},
		{prefix: "23", expect: 250},
		{prefix: "22", expect: 500},
		{prefix: "21", expect: 1000, comment: "bashible computes 1000 here, and so does every other node in the cluster"},
		{prefix: "16", expect: 1000, comment: "below the ladder bashible stays on its bottom step"},
		{prefix: "", expect: 120, comment: "bashible defaults the prefix to 24"},
	}

	for _, tt := range tests {
		t.Run("prefix "+tt.prefix, func(t *testing.T) {
			metaConfig := immutabletest.MetaConfig(t)
			if tt.prefix == "" {
				delete(metaConfig.ClusterConfig, "podSubnetNodeCIDRPrefix")
			} else {
				metaConfig.ClusterConfig["podSubnetNodeCIDRPrefix"] = json.RawMessage(`"` + tt.prefix + `"`)
			}

			nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{
				NodeName:   "example-master-0",
				MetaConfig: metaConfig,
			})
			require.NoError(t, err)
			require.Equal(t, tt.expect, nodeConfig.Spec.Kubelet.MaxPods, tt.comment)
		})
	}
}

// wrongDigest is what a row plants where the lookup must not land.
const wrongDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// The extensions are looked up by name in images_digests.json, where the sprig
// camelcase function has already stripped every separator: each row is one way
// that lookup can land on the wrong image, or on none.
func TestSysextDigests(t *testing.T) {
	tests := []struct {
		name    string
		images  func(packages map[string]any)
		want    extension
		wantErr []string
	}{
		{
			name:    "the installer ships no kubelet extension for this version",
			images:  func(packages map[string]any) { delete(packages, "kubeletSysext1349") },
			wantErr: []string{`no "kubelet" system extension digest for Kubernetes 1.34`},
		},
		{
			name: "the newest patch wins, and the suffix compares as a number",
			images: func(packages map[string]any) {
				packages["kubeletSysext13410"] = immutabletest.KubeletDigest
				packages["kubeletSysext1349"] = wrongDigest
			},
			want: extension{Name: kubeletExtension, Digest: immutabletest.KubeletDigest, RequestedBy: platformExtensionRequestedBy},
		},
		{
			// "kubernetesCniSysext1610" is 1.6.10, 1.61.0 and 16.1.0 at once; no
			// comparison gets all three readings right, so two candidates are
			// refused instead of silently resolved.
			name: "two candidates whose names do not say which one is newer",
			images: func(packages map[string]any) {
				packages["kubernetesCniSysext1610"] = immutabletest.CNIDigest
				packages["kubernetesCniSysext170"] = wrongDigest
				delete(packages, "kubernetesCniSysext162")
			},
			wantErr: []string{"kubernetesCniSysext1610, kubernetesCniSysext170", "which one is newer"},
		},
		{
			name:   "an image whose name merely starts the same way is not a candidate",
			images: func(packages map[string]any) { packages["containerdSysextArtifact224"] = wrongDigest },
			want:   extension{Name: containerdExtension, Digest: immutabletest.ContainerdDigest, RequestedBy: platformExtensionRequestedBy},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := immutabletest.MetaConfig(t)
			tt.images(metaConfig.Images["registrypackages"])

			extensions, err := sysextExtensions(metaConfig.Images.ConvertToMap(), "1.34")
			if len(tt.wantErr) == 0 {
				require.NoError(t, err)
				require.Contains(t, extensions, tt.want)
				return
			}

			require.Error(t, err)
			for _, fragment := range tt.wantErr {
				require.Contains(t, err.Error(), fragment)
			}
			// The same failure the preflight check reports, before anything is created.
			require.Error(t, ValidateSysext(t.Context(), metaConfig))
		})
	}
}

// The document dhctl writes must list every extension node-controller renders.
// A node prunes the installed extensions its document does not list, and pruning
// nodelet runs "systemctl stop nodelet.service" — the agent stops itself, and an
// explicitly stopped unit is not restarted, so the node loses its API proxy for
// good. Measured on zykov-ab-19be2a59 (13.08.2026): "stop nodelet.service before
// pruning extension nodelet: signal: killed".
func TestNodeConfigListsEveryExtensionNodeControllerRenders(t *testing.T) {
	nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{
		NodeName:   "example-master-0",
		MetaConfig: immutabletest.MetaConfig(t),
	})
	require.NoError(t, err)

	var names []string
	for _, e := range nodeConfig.Spec.Extensions {
		names = append(names, e.Name)
	}
	require.ElementsMatch(t,
		[]string{containerdExtension, kubeletExtension, cniExtension, nodeletExtension}, names)
}

// The sandbox image reference is built from the configured registry, not from
// the raw imagesRepo: the trailing-slash row shows why, as ".../ce/@sha256:…"
// is not a reference containerd can pull. The OS image needs no such assembly —
// it travels as a bare digest.
func TestNodeConfigImageReferencesFollowTheConfiguredRegistry(t *testing.T) {
	tests := []struct {
		name        string
		imagesRepo  string
		wantAddress string
		wantPath    string
		wantSandbox string
	}{
		{
			name:        "address, port and path",
			imagesRepo:  "registry.internal.example.com:5000/mirror/deckhouse",
			wantAddress: "registry.internal.example.com:5000",
			wantPath:    "/mirror/deckhouse",
			wantSandbox: "registry.internal.example.com:5000/mirror/deckhouse@" + immutabletest.PauseDigest,
		},
		{
			name:        "a trailing slash the schema lets through",
			imagesRepo:  "registry.example.com/deckhouse/ce/",
			wantAddress: "registry.example.com",
			wantPath:    "/deckhouse/ce",
			wantSandbox: "registry.example.com/deckhouse/ce@" + immutabletest.PauseDigest,
		},
		{
			name:        "no path at all",
			imagesRepo:  "registry.example.com",
			wantAddress: "registry.example.com",
			wantPath:    "",
			wantSandbox: "registry.example.com@" + immutabletest.PauseDigest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := immutabletest.MetaConfig(t)
			metaConfig.Registry.Settings.RemoteData.ImagesRepo = tt.imagesRepo

			nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{
				NodeName:   "example-master-0",
				MetaConfig: metaConfig,
			})
			require.NoError(t, err)

			require.Equal(t, tt.wantAddress, nodeConfig.Spec.Registry.Address)
			require.Equal(t, tt.wantPath, nodeConfig.Spec.Registry.Path)
			require.Equal(t, tt.wantSandbox, nodeConfig.Spec.ContainerRuntime.SandboxImage)
		})
	}
}

// The mount gives a control-plane node its etcd disk, so the three things that
// make it work are asserted directly: blank (a cloud disk has no partition
// table), the static pod's hostPath, and the mode etcd checks on every start.
func TestEtcdMountClaimsABlankDiskUnderEtcd(t *testing.T) {
	mounts := etcdMounts

	require.Len(t, mounts, 1)
	require.Equal(t, "/var/lib/etcd", mounts[0].BindTo)
	require.Equal(t, "0700", mounts[0].Mode)
	require.Equal(t, "10Gi", mounts[0].PartitionSelector.Size)
	require.True(t, mounts[0].PartitionSelector.Blank)
	require.LessOrEqual(t, len(mounts[0].Name), 16, "the name becomes an ext4 label")
}
