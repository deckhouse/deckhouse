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

package virtualcontrolplaneconfiguration

import (
	"testing"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The fixtures below mirror the placeholders the real templates in
// modules/040-control-plane-manager/virtual-control-plane-templates/ actually carry. They are
// deliberately synthetic: this test covers the replacer contract, not the template contents. Note
// the cilium templates carry no networking placeholder at all — with ipam: kubernetes the tenant
// pod CIDR reaches cilium via node.spec.podCIDR, allocated by kube-controller-manager.
const sampleAPIServerTemplate = `
- --service-cluster-ip-range=${SERVICE_SUBNET_CIDR}
- --service-account-issuer=https://kubernetes.default.svc.${CLUSTER_DOMAIN}
`

const sampleKCMTemplate = `
- --service-cluster-ip-range=${SERVICE_SUBNET_CIDR}
- --allocate-node-cidrs=true
- --cluster-cidr=${POD_SUBNET_CIDR}
- --node-cidr-mask-size=${POD_SUBNET_NODE_CIDR_PREFIX}
`

func testGlobalData() map[string][]byte {
	return map[string][]byte{
		"images": []byte(`
versioned:
  "1.31":
    apiserver: apiserver:1.31
    controllerManager: controller-manager:1.31
    scheduler: scheduler:1.31
fixed:
  kine: kine:latest
  konnectivityServer: konnectivity-server:latest
  konnectivityAgent: konnectivity-agent:latest
  cilium: cilium:latest
  ciliumOperator: cilium-operator:latest
  bashibleApiserver: bashible-apiserver:latest
`),
		"cluster-uuid":                     []byte("11111111-2222-3333-4444-555555555555"),
		"minget":                           []byte("bWluZ2V0"),
		"kube-apiserver.yaml.tpl":          []byte(sampleAPIServerTemplate),
		"kube-controller-manager.yaml.tpl": []byte(sampleKCMTemplate),
		"some-non-template-key.yaml":       []byte("should never appear in the rendered output"),
	}
}

func testVCP(networking controlplanev1alpha1.VirtualControlPlaneNetworking) *controlplanev1alpha1.VirtualControlPlane {
	return &controlplanev1alpha1.VirtualControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "acme", Namespace: "vcp-acme"},
		Spec: controlplanev1alpha1.VirtualControlPlaneSpec{
			KubernetesVersion: "1.31",
			Replicas:          1,
			Networking:        networking,
		},
	}
}

func TestRenderManifestsSubstitutesNetworkingPlaceholders(t *testing.T) {
	for _, tc := range []struct {
		name       string
		networking controlplanev1alpha1.VirtualControlPlaneNetworking
	}{
		{
			name: "default-shaped values",
			networking: controlplanev1alpha1.VirtualControlPlaneNetworking{
				ServiceSubnetCIDR:       "10.96.0.0/12",
				PodSubnetCIDR:           "10.244.0.0/16",
				PodSubnetNodeCIDRPrefix: "24",
				ClusterDomain:           "cluster.virtual",
			},
		},
		{
			name: "custom CIDRs and domain",
			networking: controlplanev1alpha1.VirtualControlPlaneNetworking{
				ServiceSubnetCIDR:       "192.168.0.0/16",
				PodSubnetCIDR:           "10.111.0.0/16",
				PodSubnetNodeCIDRPrefix: "25",
				ClusterDomain:           "example.internal",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vcp := testVCP(tc.networking)

			rendered, err := renderManifests(testGlobalData(), vcp, "1.2.3.4", nil)
			require.NoError(t, err)

			apiserverManifest := string(rendered["kube-apiserver.yaml.tpl"])
			assert.Contains(t, apiserverManifest, "--service-cluster-ip-range="+tc.networking.ServiceSubnetCIDR)
			assert.Contains(t, apiserverManifest, "--service-account-issuer=https://kubernetes.default.svc."+tc.networking.ClusterDomain)

			kcmManifest := string(rendered["kube-controller-manager.yaml.tpl"])
			assert.Contains(t, kcmManifest, "--service-cluster-ip-range="+tc.networking.ServiceSubnetCIDR)
			assert.Contains(t, kcmManifest, "--cluster-cidr="+tc.networking.PodSubnetCIDR)
			assert.Contains(t, kcmManifest, "--node-cidr-mask-size="+tc.networking.PodSubnetNodeCIDRPrefix)

			// No placeholder may survive rendering — an unreplaced ${...} would reach the tenant
			// verbatim and crashloop the component.
			assert.NotContains(t, apiserverManifest, "${")
			assert.NotContains(t, kcmManifest, "${")
		})
	}
}

// TestRenderManifestsWhitelistNotWidened locks down the non-template copy whitelist in
// renderManifests: "images", "cluster-uuid" and "minget" come from globalData (the management
// cluster's rendered config) and nothing else does. A key that isn't a *.yaml.tpl/*.sh.tpl
// template and isn't in that whitelist must not survive into the rendered Secret. renderManifests
// adds nothing of its own: the networking values live on the VirtualControlPlane and are resolved
// from it at every use, never persisted here as a second copy.
func TestRenderManifestsWhitelistNotWidened(t *testing.T) {
	vcp := testVCP(controlplanev1alpha1.VirtualControlPlaneNetworking{
		ServiceSubnetCIDR:       "10.96.0.0/12",
		PodSubnetCIDR:           "10.244.0.0/16",
		PodSubnetNodeCIDRPrefix: "24",
		ClusterDomain:           "cluster.virtual",
	})

	rendered, err := renderManifests(testGlobalData(), vcp, "1.2.3.4", nil)
	require.NoError(t, err)

	allowedExtra := map[string]struct{}{
		"images":       {},
		"cluster-uuid": {},
		"minget":       {},
	}

	for key := range rendered {
		if key == "kube-apiserver.yaml.tpl" || key == "kube-controller-manager.yaml.tpl" {
			continue
		}
		_, ok := allowedExtra[key]
		assert.True(t, ok, "unexpected key %q leaked into the rendered config secret", key)
	}

	assert.NotContains(t, rendered, "some-non-template-key.yaml")
}

// TestRenderManifestsSubstitutesVerbatim pins the post-refactor contract: renderManifests does not
// validate. The CRD schema is the only gate on spec.networking (patterns, size bounds and the
// podSubnetNodeCIDRPrefix > podSubnetCIDR prefix rule), so anything that reaches this function has
// already passed admission and is substituted as-is. The test exists so that a future reader does
// not mistake the absence of validation here for an oversight.
func TestRenderManifestsSubstitutesVerbatim(t *testing.T) {
	vcp := testVCP(controlplanev1alpha1.VirtualControlPlaneNetworking{
		ServiceSubnetCIDR:       "not-a-cidr",
		PodSubnetCIDR:           "10.244.0.0/16",
		PodSubnetNodeCIDRPrefix: "24",
		ClusterDomain:           "cluster.virtual",
	})

	rendered, err := renderManifests(testGlobalData(), vcp, "1.2.3.4", nil)
	require.NoError(t, err)
	assert.Contains(t, string(rendered["kube-apiserver.yaml.tpl"]), "--service-cluster-ip-range=not-a-cidr")
}
