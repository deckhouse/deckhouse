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

package nodeconfig

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// mutableDigests are the release images a bashible node's document is rendered from, and
// nothing an Engine node needs.
func mutableDigests() map[string]string {
	digests := map[string]string{"nodelet": testNodeletPackageDigest}
	maps.Copy(digests, testenv.TestPlatformPackageDigests)
	return digests
}

// digestsJSON is images_digests.json carrying these registry packages alone.
func digestsJSON(t *testing.T, packages map[string]string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]map[string]string{registryPackagesDigestsKey: packages})
	require.NoError(t, err)
	return string(raw)
}

// testPlatform is the platform document of this checkout at the test digests.
func testPlatform(t require.TestingT) platformDocument {
	platform, err := renderPlatformDocument(testenv.PlatformDocument(),
		map[string]map[string]string{registryPackagesDigestsKey: mutableDigests()})
	require.NoError(t, err)
	return platform
}

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
  staticPods:
  - name: early
    manifest: "kind: Pod"
`

// The platform's part of a bashible node's document is the platform document with the
// release's digests in it, every entry marked as the platform's.
func TestThePlatformDocumentRendersWithTheReleaseDigests(t *testing.T) {
	platform, err := renderPlatformDocument(twoPackages, map[string]map[string]string{
		registryPackagesDigestsKey: {"nodelet": "sha256:agent", "jq": "sha256:jq"},
	})
	require.NoError(t, err)

	require.Equal(t, platformDocument{
		FeatureGates: map[string]bool{"extensions": true},
		Extensions: []internalv1alpha1.Extension{
			{Name: "nodelet", Digest: "sha256:agent", RequestedBy: platformExtensionRequestedBy},
			{Name: "jq", Digest: "sha256:jq", RequestedBy: platformExtensionRequestedBy},
		},
		StaticPods: []internalv1alpha1.StaticPod{{Name: "early", Manifest: "kind: Pod"}},
	}, platform)
}

// Skipped silently, a package the release does not list would be waited for on a
// bashible step with no reason in any status: the render fails instead.
func TestThePlatformDocumentRefusesAReleaseMissingAPackage(t *testing.T) {
	_, err := renderPlatformDocument(twoPackages, map[string]map[string]string{
		registryPackagesDigestsKey: {"nodelet": "sha256:agent"},
	})

	require.ErrorContains(t, err, "jq")
}

// A field the Mutable render does not carry would be dropped without a word.
func TestThePlatformDocumentRefusesAFieldItDoesNotCarry(t *testing.T) {
	_, err := renderPlatformDocument(twoPackages+"  kubelet: {}\n", map[string]map[string]string{
		registryPackagesDigestsKey: {"nodelet": "sha256:agent", "jq": "sha256:jq"},
	})

	require.ErrorContains(t, err, "kubelet")
}

// The document the release ships renders, starts with the agent and names each package once.
func TestTheShippedPlatformDocumentRenders(t *testing.T) {
	platform := testPlatform(t)

	require.Equal(t, internalv1alpha1.Extension{
		Name: nodeletExtension, Digest: testNodeletPackageDigest, RequestedBy: platformExtensionRequestedBy,
	}, platform.Extensions[0])
	require.Len(t, platform.Extensions, len(testenv.TestPlatformPackageDigests)+1)
	seen := map[string]bool{}
	for _, ext := range platform.Extensions {
		require.False(t, seen[ext.Name], "%s is listed twice", ext.Name)
		seen[ext.Name] = true
	}
}

// A bashible node gets the platform document of the release, read from the ConfigMap
// helm ships it in.
func TestReadMutableInputsTakesThePlatformDocument(t *testing.T) {
	cluster := mutableCluster(t, digestsJSON(t, mutableDigests()))

	in, err := sourceReaderOver(cluster).readMutableInputs(t.Context())
	require.NoError(t, err)

	require.Equal(t, testPlatform(t), in.Platform)
}

func TestReadMutableInputsRefusesAReleaseMissingAPlatformPackage(t *testing.T) {
	digests := mutableDigests()
	delete(digests, "jq")
	cluster := mutableCluster(t, digestsJSON(t, digests))

	_, err := sourceReaderOver(cluster).readMutableInputs(t.Context())

	require.ErrorContains(t, err, "jq")
}

// The documents already written stay as they were while the release lacks a package.
func TestABashibleNodeKeepsItsDocumentWhileAPlatformPackageIsMissing(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	node := nodeInGroup("worker-0", "worker")
	existing := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Labels: map[string]string{
			nodecommon.NodeGroupLabel: "worker", managedByLabel: managedByValue,
		}},
		Spec: internalv1alpha1.NodeSpec{SystemType: internalv1alpha1.SystemTypeMutable, NodeName: "worker-0"},
	}
	digests := mutableDigests()
	delete(digests, "socat1734")
	cl := mutableCluster(t, digestsJSON(t, digests), ng, &node, existing)
	r := &Reconciler{}
	r.Client = cl
	r.sources = sourceReaderOver(cl)

	_, err := r.reconcileNodeObject(t.Context(), &node, logr.Discard(), newPass())
	require.ErrorContains(t, err, "socat1734")

	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: "worker-0"}, stored))
	require.Equal(t, existing.Spec, stored.Spec)
}

// Every edition builds the agent, so the platform document is read for every release,
// and one without the agent's package fails it like any other missing package.
func TestThePlatformDocumentRefusesAReleaseWithoutTheAgentPackage(t *testing.T) {
	digests := mutableDigests()
	delete(digests, "nodelet")
	cluster := mutableCluster(t, digestsJSON(t, digests))

	_, err := sourceReaderOver(cluster).readPlatformDocument(t.Context(),
		map[string]map[string]string{registryPackagesDigestsKey: digests})

	require.ErrorContains(t, err, `"nodelet"`)
}

// The platform document is the bashible nodes' own: an Engine node is rendered without
// it, so a release that breaks it holds back no Engine node.
func TestAnEngineNodeIsRenderedWhileAPlatformPackageIsMissing(t *testing.T) {
	var digests map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(testenv.ImagesDigestsJSON(testenv.TestContainerdDigest)), &digests))
	delete(digests[registryPackagesDigestsKey], "jq")
	raw, err := json.Marshal(digests)
	require.NoError(t, err)
	sysext := ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
		Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension},
	})
	cluster := mutableCluster(t, string(raw), &sysext,
		dnsService(kubeDNSServiceName, "kube-dns", "10.0.0.10"),
		clusterCAConfigMapObject(testenv.TestClusterCA),
		clusterConfigSecret("clusterDomain: cluster.local\n"),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: d8SystemNS, Name: deckhouseRegistrySecret},
			Data:       map[string][]byte{registryAddressKey: []byte(testenv.TestRegistryAddress)},
		},
	)

	in, err := sourceReaderOver(cluster).readClusterInputs(t.Context(), testenv.TestKubernetesVersion)
	require.NoError(t, err)

	require.Equal(t, []string{"drbd"}, requestedNames(renderSpec(immutableGroup("worker"), nodeWith(nil), in).Extensions))
}

// The file is the bashible nodes' document: one written for another kind of node is
// refused, not poured into a Mutable spec.
func TestThePlatformDocumentRefusesANodeThatIsNotMutable(t *testing.T) {
	_, err := renderPlatformDocument(strings.Replace(twoPackages, "systemType: Mutable", "systemType: Immutable", 1),
		map[string]map[string]string{registryPackagesDigestsKey: {"nodelet": "sha256:agent", "jq": "sha256:jq"}})

	require.ErrorContains(t, err, `"Immutable" node`)
}
