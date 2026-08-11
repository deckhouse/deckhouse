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

// TestRenderControlPlaneBundleOrdersEtcdFirst pins the order the node writes in.
// kubelet starts a static pod the moment its manifest appears, and an apiserver
// that comes up before its datastore only crash-loops until etcd catches up.
func TestRenderControlPlaneBundleOrdersEtcdFirst(t *testing.T) {
	manifests, extraFiles, err := renderControlPlaneBundle(t.Context(), testManifestsInput(t))
	require.NoError(t, err)

	require.Equal(t, "etcd.yaml", manifests[0].Name)
	require.Len(t, manifests, 4)
	require.Len(t, extraFiles, 1)
	require.Equal(t, "authentication-config.yaml", extraFiles[0].Name)
}

// TestRenderControlPlaneBundleLeavesTheAddressToTheNode covers the one thing
// the installer cannot know: the node's address travels as a placeholder and
// every manifest that needs it keeps it verbatim.
func TestRenderControlPlaneBundleLeavesTheAddressToTheNode(t *testing.T) {
	in := testManifestsInput(t)
	in.NodeIP = nodeAddressPlaceholder

	manifests, extraFiles, err := renderControlPlaneBundle(t.Context(), in)
	require.NoError(t, err)

	withPlaceholder := 0
	for _, file := range append(manifests, extraFiles...) {
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

// A manifest kubelet cannot parse is a static pod that never appears at all,
// with the reason only in the kubelet log of a node nobody can log in to.
func TestRenderedManifestsAreValidPods(t *testing.T) {
	manifests, _, err := renderControlPlaneBundle(t.Context(), testManifestsInput(t))
	require.NoError(t, err)

	for _, file := range manifests {
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
	manifests, extraFiles, err := renderControlPlaneBundle(t.Context(), testManifestsInput(t))
	require.NoError(t, err)
	require.NotEmpty(t, extraFiles, "the bootstrap needs the authentication config")

	var apiserver string
	for _, file := range manifests {
		if file.Name == "kube-apiserver.yaml" {
			apiserver = file.Content
		}
	}
	for _, file := range extraFiles {
		require.Contains(t, apiserver, "--authentication-config="+extraFilesDir+"/"+file.Name)
	}
}

// Every one of these flags names a file that only exists after the module
// preparators have run, and nothing runs them on a node bringing a cluster up.
// A component started with one of them exits before it opens a port.
func TestRenderOmitsFlagsWhoseFilesDoNotExistYet(t *testing.T) {
	manifests, _, err := renderControlPlaneBundle(t.Context(), testManifestsInput(t))
	require.NoError(t, err)

	rendered := make(map[string]string, len(manifests))
	for _, file := range manifests {
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
	in := testManifestsInput(t)
	in.Cluster.ClusterType = ""

	manifests, _, err := renderControlPlaneBundle(t.Context(), in)
	require.NoError(t, err)

	for _, file := range manifests {
		if file.Name == "kube-controller-manager.yaml" {
			require.NotContains(t, file.Content, "--cloud-provider",
				"an unset clusterType must not enable the external cloud provider")
		}
	}
}

// testCandiDir finds the installer's candi directory from wherever the test is
// run, so the render reads the same templates the classic bootstrap renders.
func testCandiDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		candidate := filepath.Join(dir, "candi")
		if _, err := os.Stat(filepath.Join(candidate, controlPlaneTemplatesDir)); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "candi/%s not found above %s", controlPlaneTemplatesDir, dir)
		dir = parent
	}
}

func testManifestsInput(t *testing.T) manifestsInput {
	return manifestsInput{
		CandiDir: testCandiDir(t),
		NodeName: "master-0",
		NodeIP:   "10.241.32.10",
		Cluster: controlPlaneRenderParams{
			clusterParamsSpec: clusterParamsSpec{
				ClusterDomain: "cluster.local", ServiceSubnetCIDR: "10.222.0.0/16",
			},
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
