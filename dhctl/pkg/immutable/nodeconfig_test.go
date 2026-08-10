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

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

const (
	testContainerdDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testCNIDigest        = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testKubeletDigest    = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	testPauseDigest      = "sha256:4444444444444444444444444444444444444444444444444444444444444444"

	testEtcdDigest              = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	testAPIServerDigest         = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	testControllerManagerDigest = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	testSchedulerDigest         = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
)

func testMetaConfig(t *testing.T) *config.MetaConfig {
	t.Helper()

	const masterNodeGroup = `{
	  "replicas": 1,
	  "instanceClass": {
	    "rootDisk": {"size": "50Gi"},
	    "etcdDisk": {"size": "10Gi"}
	  }
	}`

	metaConfig := &config.MetaConfig{
		ClusterType:       config.CloudClusterType,
		ClusterPrefix:     "example",
		ClusterDomain:     "cluster.local",
		ClusterDNSAddress: "10.223.0.10",
		ClusterConfig: map[string]json.RawMessage{
			"kubernetesVersion":       json.RawMessage(`"1.34"`),
			"serviceSubnetCIDR":       json.RawMessage(`"10.223.0.0/16"`),
			"podSubnetCIDR":           json.RawMessage(`"10.222.0.0/16"`),
			"podSubnetNodeCIDRPrefix": json.RawMessage(`"24"`),
			"clusterDomain":           json.RawMessage(`"cluster.local"`),
		},
		ProviderClusterConfig: map[string]json.RawMessage{
			"masterNodeGroup": json.RawMessage(masterNodeGroup),
		},
		Images: map[string]map[string]any{
			"registrypackages": {
				"containerdSysext224":    testContainerdDigest,
				"kubernetesCniSysext162": testCNIDigest,
				"kubeletSysext1349":      testKubeletDigest,
				"kubeletSysext1336":      "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			},
			"common": {
				"pause": testPauseDigest,
			},
			"controlPlaneManager": {
				"etcd":                     testEtcdDigest,
				"kubeApiserver134":         testAPIServerDigest,
				"kubeControllerManager134": testControllerManagerDigest,
				"kubeScheduler134":         testSchedulerDigest,
			},
		},
	}

	metaConfig.Registry.Settings = registry.ModeSettings{
		Mode: constant.ModeUnmanaged,
		RemoteData: registry.Data{
			ImagesRepo: "dev-registry.deckhouse.io/sys/deckhouse-oss",
			Scheme:     constant.SchemeHTTPS,
			Username:   "user",
			Password:   "password",
		},
	}

	return metaConfig
}

// maxPods is a function of how many addresses a node's slice of the pod subnet
// holds, and bashible computes it that way for every other node in the cluster
// (064_configure_kubelet.sh.tpl). A first master that used a flat default would
// advertise 120 on a /22 cluster where every bashible node advertises 500, and
// the scheduler believes both.
//
// The ladder is then capped at what the nodeConfig schema accepts, exactly as
// node-controller caps it, so the first day-2 render of this node writes the
// number it already booted with rather than a spec diff and a rollout slot.
func TestNodeConfigMaxPodsFollowsThePodSubnet(t *testing.T) {
	const cappedFrom1000 = "bashible computes 1000 here; the nodeConfig schema caps it at 500"

	tests := []struct {
		prefix  string
		expect  int
		comment string
	}{
		{prefix: "26", expect: 120},
		{prefix: "24", expect: 120},
		{prefix: "23", expect: 250},
		{prefix: "22", expect: 500},
		{prefix: "21", expect: 500, comment: cappedFrom1000},
		{prefix: "20", expect: 500, comment: cappedFrom1000},
		{prefix: "", expect: 120, comment: "bashible defaults the prefix to 24"},
	}

	for _, tt := range tests {
		t.Run("prefix "+tt.prefix, func(t *testing.T) {
			metaConfig := testMetaConfig(t)
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

func TestSysextDigestsMissing(t *testing.T) {
	metaConfig := testMetaConfig(t)
	delete(metaConfig.Images["registrypackages"], "kubeletSysext1349")

	_, err := SysextDigests(t.Context(), metaConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), `no "kubelet" system extension digest for Kubernetes 1.34`)
}

func TestSysextDigestsPicksNewestPatch(t *testing.T) {
	metaConfig := testMetaConfig(t)
	metaConfig.Images["registrypackages"]["kubeletSysext13410"] = testKubeletDigest
	metaConfig.Images["registrypackages"]["kubeletSysext1349"] = "sha256:9999999999999999999999999999999999999999999999999999999999999999"

	digests, err := SysextDigests(t.Context(), metaConfig)
	require.NoError(t, err)
	require.Equal(t, testKubeletDigest, digests["kubelet"])
}

// The camelcase function that builds the digest map strips the separators out
// of the version, so "kubernetesCniSysext1610" is 1.6.10, 1.61.0 and 16.1.0 at
// once. A numeric compare on it reads 1610 > 170 and installs CNI 1.6.10 over
// 1.7.0; there is no comparison that gets all three readings right, so two
// candidates are refused instead of silently resolved.
func TestSysextDigestsRefusesAmbiguousVersions(t *testing.T) {
	metaConfig := testMetaConfig(t)
	metaConfig.Images["registrypackages"]["kubernetesCniSysext1610"] = testCNIDigest
	metaConfig.Images["registrypackages"]["kubernetesCniSysext170"] = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
	delete(metaConfig.Images["registrypackages"], "kubernetesCniSysext162")

	_, err := SysextDigests(t.Context(), metaConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kubernetesCniSysext1610, kubernetesCniSysext170")
	require.Contains(t, err.Error(), "which one is newer")
}

// The one containerd and the one CNI extension the installer ships are found by
// their prefix alone, and an image whose name merely starts the same way is not
// one of them.
func TestSysextDigestsIgnoresNonVersionSuffixes(t *testing.T) {
	metaConfig := testMetaConfig(t)
	metaConfig.Images["registrypackages"]["containerdSysextArtifact224"] = "sha256:9999999999999999999999999999999999999999999999999999999999999999"

	digests, err := SysextDigests(t.Context(), metaConfig)
	require.NoError(t, err)
	require.Equal(t, testContainerdDigest, digests["containerd"])
}

// Both image references the node pulls before it has a cluster — its own OS and
// the pause image every pod sandbox starts from — are built from the registry it
// was given, not from the public one and not from the raw imagesRepo. A private
// or air-gapped registry is the normal case here, and a first master that cannot
// pull either has no cluster yet in which to be fixed.
//
// The trailing-slash row is the reason the raw value is the wrong input: the
// InitConfiguration pattern admits it (the path class contains "/"), and
// ".../ce/@sha256:…" is not a reference containerd can pull.
func TestNodeConfigImageReferencesFollowTheConfiguredRegistry(t *testing.T) {
	tests := []struct {
		name        string
		imagesRepo  string
		wantAddress string
		wantPath    string
		wantOSImage string
		wantSandbox string
	}{
		{
			name:        "address, port and path",
			imagesRepo:  "registry.internal.example.com:5000/mirror/deckhouse",
			wantAddress: "registry.internal.example.com:5000",
			wantPath:    "/mirror/deckhouse",
			wantOSImage: "registry.internal.example.com:5000/mirror/deckhouse/" + osImageNameAndTag,
			wantSandbox: "registry.internal.example.com:5000/mirror/deckhouse@" + testPauseDigest,
		},
		{
			name:        "a trailing slash the schema lets through",
			imagesRepo:  "registry.example.com/deckhouse/ce/",
			wantAddress: "registry.example.com",
			wantPath:    "/deckhouse/ce",
			wantOSImage: "registry.example.com/deckhouse/ce/" + osImageNameAndTag,
			wantSandbox: "registry.example.com/deckhouse/ce@" + testPauseDigest,
		},
		{
			name:        "no path at all",
			imagesRepo:  "registry.example.com",
			wantAddress: "registry.example.com",
			wantPath:    "",
			wantOSImage: "registry.example.com/" + osImageNameAndTag,
			wantSandbox: "registry.example.com@" + testPauseDigest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := testMetaConfig(t)
			metaConfig.Registry.Settings.RemoteData.ImagesRepo = tt.imagesRepo

			nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{
				NodeName:   "example-master-0",
				MetaConfig: metaConfig,
			})
			require.NoError(t, err)

			require.Equal(t, tt.wantAddress, nodeConfig.Spec.Registry.Address)
			require.Equal(t, tt.wantPath, nodeConfig.Spec.Registry.Path)
			require.Equal(t, tt.wantOSImage, nodeConfig.Spec.OSImage)
			require.Equal(t, tt.wantSandbox, nodeConfig.Spec.ContainerRuntime.SandboxImage)
		})
	}
}

// The mount is what gives a control-plane node its etcd disk, so the three
// things that make it work are asserted rather than left to the golden file:
// blank (a cloud disk has no partition table, so nothing else would match it),
// the path etcd's static pod carries as a hostPath, and the mode etcd checks on
// every start.
func TestEtcdMountClaimsABlankDiskUnderEtcd(t *testing.T) {
	mounts := etcdMounts()

	require.Len(t, mounts, 1)
	require.Equal(t, etcdDataDir, mounts[0].BindTo)
	require.Equal(t, etcdDataMode, mounts[0].Mode)
	require.Equal(t, etcdDiskSize, mounts[0].PartitionSelector.Size)
	require.True(t, mounts[0].PartitionSelector.Blank)
	require.LessOrEqual(t, len(mounts[0].Name), 16, "the name becomes an ext4 label")
}
