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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// TestRenderControlPlaneBundleMatchesTheAgentsRender is the parity proof for
// moving the render off the node.
//
// The files under testdata/controlplane were produced by the renderer this one
// replaces — the agent's internal/controllers/controlplane/manifests package, at
// the commit before it was deleted — with the inputs each case names. They are
// therefore an oracle rather than a snapshot of this code: a golden taken from
// the engine under test would guard nothing.
//
// What the parity is worth: the first master's control plane is the cluster's,
// and a difference of one byte between what the installer renders and what the
// node used to render is a control-plane component started with something else.
func TestRenderControlPlaneBundleMatchesTheAgentsRender(t *testing.T) {
	cases := []struct {
		name string
		in   manifestsInput
	}{
		{"cloud-134", manifestsInput{
			NodeName: "master-0", NodeIP: "10.241.32.10",
			Cluster: controlPlaneRenderParams{
				ClusterDomain: "cluster.local", ServiceSubnetCIDR: "10.222.0.0/16",
				PodSubnetCIDR: "10.111.0.0/16", PodSubnetNodeCIDRPrefix: "24",
				KubernetesVersion: "1.34", ClusterType: "Cloud",
			},
			Registry: &registrySpec{Address: "registry.deckhouse.io", Path: "/deckhouse/ce"},
			Images: controlPlaneImages{
				Etcd: "sha256:1111", KubeAPIServer: "sha256:2222",
				KubeControllerManager: "sha256:3333", KubeScheduler: "sha256:4444",
			},
		}},
		{"static-131", manifestsInput{
			NodeName: "control-plane-1", NodeIP: "192.168.199.2",
			Cluster: controlPlaneRenderParams{
				ClusterDomain: "k8s.internal", ServiceSubnetCIDR: "10.96.0.0/12",
				PodSubnetCIDR: "10.244.0.0/16", PodSubnetNodeCIDRPrefix: "22",
				KubernetesVersion: "1.31", ClusterType: "Static",
			},
			Registry: &registrySpec{Address: "mirror.example.com:5000", Path: "/deckhouse/ee"},
			Images: controlPlaneImages{
				Etcd: "sha256:aaaa", KubeAPIServer: "sha256:bbbb",
				KubeControllerManager: "sha256:cccc", KubeScheduler: "sha256:dddd",
			},
		}},
		{"registry-root", manifestsInput{
			NodeName: "m0", NodeIP: "172.16.0.3",
			Cluster: controlPlaneRenderParams{
				ClusterDomain: "cluster.local", ServiceSubnetCIDR: "10.222.0.0/16",
				PodSubnetCIDR: "10.111.0.0/16", PodSubnetNodeCIDRPrefix: "21",
				KubernetesVersion: "1.33", ClusterType: "Cloud",
			},
			Registry: &registrySpec{Address: "10.0.0.1:5000", Path: ""},
			Images: controlPlaneImages{
				Etcd: "sha256:e", KubeAPIServer: "sha256:a",
				KubeControllerManager: "sha256:c", KubeScheduler: "sha256:s",
			},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := renderControlPlaneBundle(t.Context(), tc.in)
			require.NoError(t, err)

			dir := filepath.Join("testdata", "controlplane", tc.name)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)

			rendered := make(map[string]string, len(bundle.Manifests)+len(bundle.ExtraFiles))
			for _, file := range append(append([]renderedFile{}, bundle.Manifests...), bundle.ExtraFiles...) {
				rendered[file.Name] = file.Content
			}
			require.Len(t, rendered, len(entries), "the render produced a different set of files")

			for _, entry := range entries {
				want, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				require.NoError(t, err)
				require.Equal(t, string(want), rendered[entry.Name()], "%s differs from the agent's render", entry.Name())
			}
		})
	}
}

// TestRenderControlPlaneBundleOrdersEtcdFirst pins the order the node writes in.
// kubelet starts a static pod the moment its manifest appears, and an apiserver
// that comes up before its datastore only crash-loops until etcd catches up.
func TestRenderControlPlaneBundleOrdersEtcdFirst(t *testing.T) {
	bundle, err := renderControlPlaneBundle(t.Context(), testManifestsInput())
	require.NoError(t, err)

	require.Equal(t, "etcd.yaml", bundle.Manifests[0].Name)
	require.Len(t, bundle.Manifests, 4)
	require.Len(t, bundle.ExtraFiles, 1)
	require.Equal(t, "authentication-config.yaml", bundle.ExtraFiles[0].Name)
}

// TestRenderControlPlaneBundleLeavesTheAddressToTheNode covers the one thing the
// installer cannot know: the payload is built before the machine exists, so the
// node's address travels as a placeholder and every manifest that needs it keeps
// it verbatim.
func TestRenderControlPlaneBundleLeavesTheAddressToTheNode(t *testing.T) {
	in := testManifestsInput()
	in.NodeIP = nodeAddressPlaceholder

	bundle, err := renderControlPlaneBundle(t.Context(), in)
	require.NoError(t, err)

	withPlaceholder := 0
	for _, file := range append(append([]renderedFile{}, bundle.Manifests...), bundle.ExtraFiles...) {
		if strings.Contains(file.Content, nodeAddressPlaceholder) {
			withPlaceholder++
		}
		// The node replaces exactly this token, so no other shell-looking
		// expansion may survive the render — it would reach the node as a
		// literal and the component would fail on its own command line.
		for _, line := range strings.Split(file.Content, "\n") {
			rest := line
			for {
				before, after, found := strings.Cut(rest, "$")
				_ = before
				if !found {
					break
				}
				require.True(t, strings.HasPrefix(after, "MY_IP"),
					"%s carries an expansion the node does not substitute: $%s", file.Name, after)
				rest = after
			}
		}
	}
	require.Equal(t, 2, withPlaceholder, "etcd and kube-apiserver are the manifests built around the node's own address")
}

// TestRenderControlPlaneBundleRefusesWhatTheNodeCannotFix pins the fail-closed
// half. Each of these reaches a node as an empty flag or an unreachable image,
// and a node with no shell has no way to say which.
func TestRenderControlPlaneBundleRefusesWhatTheNodeCannotFix(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*manifestsInput)
		wantErr string
	}{
		{"no registry", func(in *manifestsInput) { in.Registry = nil }, "registry is nil"},
		{"no registry address", func(in *manifestsInput) { in.Registry.Address = "" }, "registry.address must not be empty"},
		{"no pod subnet", func(in *manifestsInput) { in.Cluster.PodSubnetCIDR = "" }, "cluster.podSubnetCIDR must not be empty"},
		{"no apiserver image", func(in *manifestsInput) { in.Images.KubeAPIServer = "" }, "images.kubeApiserver must not be empty"},
		{"a node address that is neither", func(in *manifestsInput) { in.NodeIP = "master-0.example.com" }, `neither an IP address nor $MY_IP`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testManifestsInput()
			registry := *in.Registry
			in.Registry = &registry
			tt.mutate(&in)

			_, err := renderControlPlaneBundle(t.Context(), in)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// A manifest kubelet cannot parse is a static pod that never appears at all,
// with the reason only in the kubelet log of a node nobody can log in to.
func TestRenderedManifestsAreValidPods(t *testing.T) {
	bundle, err := renderControlPlaneBundle(t.Context(), testManifestsInput())
	require.NoError(t, err)

	for _, file := range bundle.Manifests {
		var pod corev1.Pod
		require.NoError(t, yaml.UnmarshalStrict([]byte(file.Content), &pod), "%s does not parse as a Pod", file.Name)
		require.Equal(t, "v1", pod.APIVersion, "%s", file.Name)
		require.Equal(t, "Pod", pod.Kind, "%s", file.Name)
		require.Len(t, pod.Spec.Containers, 1, "%s", file.Name)
		require.NotEmpty(t, pod.Spec.Containers[0].Image, "%s declares a container with no image", file.Name)
	}
}

// extraFilesDir is where the apiserver manifest looks for the files it names by
// path; the node writes them there.
const extraFilesDir = "/etc/kubernetes/deckhouse/extra-files"

// The apiserver names that file by path and exits at startup when it is not
// there, so the manifest and the extra files have to agree on the name. They
// come out of two separate renders, and nothing but this connects them.
func TestTheApiserverFlagNamesAFileTheExtraFilesRender(t *testing.T) {
	bundle, err := renderControlPlaneBundle(t.Context(), testManifestsInput())
	require.NoError(t, err)
	require.NotEmpty(t, bundle.ExtraFiles, "the bootstrap needs the authentication config")

	var apiserver string
	for _, file := range bundle.Manifests {
		if file.Name == "kube-apiserver.yaml" {
			apiserver = file.Content
		}
	}
	for _, file := range bundle.ExtraFiles {
		require.Contains(t, apiserver, "--authentication-config="+extraFilesDir+"/"+file.Name)
	}
}

// Every one of these flags names a file that only exists after the module
// preparators have run, and nothing runs them on a node bringing a cluster up.
// A component started with one of them exits before it opens a port.
func TestRenderOmitsFlagsWhoseFilesDoNotExistYet(t *testing.T) {
	bundle, err := renderControlPlaneBundle(t.Context(), testManifestsInput())
	require.NoError(t, err)

	rendered := make(map[string]string, len(bundle.Manifests))
	for _, file := range bundle.Manifests {
		rendered[file.Name] = file.Content
	}

	for _, forbidden := range []struct {
		manifest string
		fragment string
	}{
		{"kube-apiserver.yaml", "--admission-control-config-file"},
		{"kube-apiserver.yaml", "--enable-admission-plugins"},
		{"kube-apiserver.yaml", "--kubelet-certificate-authority"},
		{"kube-apiserver.yaml", "--encryption-provider-config"},
		{"kube-apiserver.yaml", "--audit-policy-file"},
		{"kube-apiserver.yaml", "--authorization-config"},
		{"kube-apiserver.yaml", "--authentication-token-webhook-config-file"},
		{"kube-scheduler.yaml", "scheduler-config.yaml"},
	} {
		require.NotContains(t, rendered[forbidden.manifest], forbidden.fragment,
			"%s names a file no bootstrapping node has", forbidden.manifest)
	}
}

// An unset clusterType is a template comparison against a missing key, which is
// an execution error rather than a false — the render must survive it.
func TestRenderWithoutAClusterTypeStillRenders(t *testing.T) {
	in := testManifestsInput()
	in.Cluster.ClusterType = ""

	bundle, err := renderControlPlaneBundle(t.Context(), in)
	require.NoError(t, err)

	for _, file := range bundle.Manifests {
		if file.Name == "kube-controller-manager.yaml" {
			require.NotContains(t, file.Content, "--cloud-provider",
				"an unset clusterType must not enable the external cloud provider")
		}
	}
}

func testManifestsInput() manifestsInput {
	return manifestsInput{
		NodeName: "master-0",
		NodeIP:   "10.241.32.10",
		Cluster: controlPlaneRenderParams{
			ClusterDomain: "cluster.local", ServiceSubnetCIDR: "10.222.0.0/16",
			PodSubnetCIDR: "10.111.0.0/16", PodSubnetNodeCIDRPrefix: "24",
			KubernetesVersion: "1.34", ClusterType: "Cloud",
		},
		Registry: &registrySpec{Address: "registry.deckhouse.io", Path: "/deckhouse/ce"},
		Images: controlPlaneImages{
			Etcd: "sha256:1111", KubeAPIServer: "sha256:2222",
			KubeControllerManager: "sha256:3333", KubeScheduler: "sha256:4444",
		},
	}
}
