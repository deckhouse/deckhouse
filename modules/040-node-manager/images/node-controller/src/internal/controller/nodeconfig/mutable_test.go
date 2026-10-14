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
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// testProxyToken is a placeholder, built here so no token-shaped literal sits in the tree.
var testProxyToken = base64.StdEncoding.EncodeToString([]byte("proxy-token"))

// A bashible node's document carries what the agent does there and nothing an
// Engine node needs: no OS image, no system extensions.
func TestTheMutableRenderCarriesOnlyWhatTheAgentDoesThere(t *testing.T) {
	staticPod := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	ordered := orderedNSPRs([]deckhousev1alpha1.NodeStaticPodRequest{staticPod})
	drbd := ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
		Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension},
	})
	requests := orderedNERs([]deckhousev1alpha1.NodeExtensionRequest{drbd})

	spec := renderMutableSpec(namedNode("worker-0"), "worker", clusterInputs{
		APIServerEndpoints:         []string{"https://10.0.0.1:6443"},
		RegistryPackagesProxyToken: testProxyToken,
		Platform: platformDocument{
			FeatureGates: map[string]bool{"static-pods": true, "extensions": true},
			Extensions: []internalv1alpha1.Extension{{
				Name: nodeletExtension, Digest: testNodeletPackageDigest, RequestedBy: platformExtensionRequestedBy,
			}},
		},
		NodeStaticPodRequests:         ordered,
		NodeStaticPodRequestsRejected: refusedEverywhere(ordered),
		// What an Engine render turns into extensions; never read here.
		SysextDigests:          map[string]string{containerdExtension: testContainerdDigest},
		NodeExtensions:         requests,
		NodeExtensionConflicts: resolveNERConflicts(requests, noPlatform),
		OSImage:                internalv1alpha1.OSImage{Digest: testOSImageDigest},
	})

	require.Equal(t, internalv1alpha1.SystemTypeMutable, spec.SystemType)
	require.Equal(t, "worker-0", spec.NodeName)
	require.Zero(t, spec.OSImage)
	require.Equal(t, []string{"https://10.0.0.1:6443"}, spec.APIServerEndpoints)
	require.Equal(t, testProxyToken, spec.RegistryPackagesProxyAccessTokenB64)
	require.Equal(t, map[string]bool{"static-pods": true, "extensions": true}, spec.FeatureGates)
	require.Equal(t, []internalv1alpha1.Extension{{
		Name:        "nodelet",
		Digest:      testNodeletPackageDigest,
		RequestedBy: platformExtensionRequestedBy,
	}}, spec.Extensions)
	require.Equal(t, []string{"static-pod"}, staticPodNames(spec.StaticPods))
}

// registry-agent is a bashible step's on the nodes bashible manages (053): a
// second writer of that file, or of that pod under another file, fights it. An
// Engine node has no such step, so the global reserved list stays as it is.
func TestTheMutableRenderLeavesTheRegistryAgentToBashible(t *testing.T) {
	byName := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	byPod := nspr("agent-copy", deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: kubeSystemPodManifest("registry-agent")})
	other := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	ordered := orderedNSPRs([]deckhousev1alpha1.NodeStaticPodRequest{byName, byPod, other})
	rejected, onMutable := rejectedNSPRs(ordered)

	mutable := renderMutableSpec(namedNode("worker-0"), "worker", clusterInputs{
		NodeStaticPodRequests:         ordered,
		NodeStaticPodRequestsRejected: onMutable,
	})
	require.Equal(t, []string{"static-pod"}, staticPodNames(mutable.StaticPods))

	ng := immutableGroup("worker")
	engine := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "engine-0"}}, clusterInputs{
		NodeStaticPodRequests:         ordered,
		NodeStaticPodRequestsRejected: rejected,
	})
	require.Equal(t, []string{"agent-copy", "registry-agent", "static-pod"}, staticPodNames(engine.StaticPods))
}

// The inputs of a bashible node are read without anything only an Engine node
// needs: a cluster that ships no OS image or sysext still renders its nodes.
func TestReadMutableInputsNeedsNoEngineImage(t *testing.T) {
	agent := nspr("registry-agent", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	cluster := mutableCluster(t, digestsJSON(t, mutableDigests()), &agent)

	in, err := sourceReaderOver(cluster).readMutableInputs(t.Context())
	require.NoError(t, err)

	require.Equal(t, []string{"https://192.168.199.10:6443"}, in.APIServerEndpoints)
	require.NotEmpty(t, in.RegistryPackagesProxyToken)
	require.Contains(t, in.NodeStaticPodRequestsRejected, "registry-agent", "the render of a Mutable node leaves it out")
}

// Every edition builds the agent's package, so a release without it is broken: the
// inputs of a bashible node fail on it by name rather than leave the node without an agent.
func TestReadMutableInputsRefusesAReleaseWithoutTheAgentPackage(t *testing.T) {
	digests := mutableDigests()
	delete(digests, "nodelet")
	cluster := mutableCluster(t, digestsJSON(t, digests))

	_, err := sourceReaderOver(cluster).readMutableInputs(t.Context())

	require.ErrorContains(t, err, `"nodelet"`)
}

// The rendered document is what the API server stores: a field it defaults that
// the render left out reads as drift, and every pass would patch the node and
// spend a rollout slot on nothing.
func TestTheMutableRenderIsWhatTheAPIServerStores(t *testing.T) {
	c := nodeConfigAPIClient(t)

	staticPod := nspr("static-pod", deckhousev1alpha1.NodeStaticPodRequestSpec{})
	ordered := orderedNSPRs([]deckhousev1alpha1.NodeStaticPodRequest{staticPod})
	desired := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bashible-0"},
		Spec: renderMutableSpec(namedNode("bashible-0"), "worker", clusterInputs{
			APIServerEndpoints:            []string{"https://10.0.0.1:6443"},
			RegistryPackagesProxyToken:    testProxyToken,
			NodeStaticPodRequests:         ordered,
			NodeStaticPodRequestsRejected: refusedEverywhere(ordered),
		}),
	}
	require.Zero(t, desired.Spec.Kubelet, "a Mutable document sends no kubelet section to default")
	require.Zero(t, desired.Spec.ContainerRuntime, "nor a containerRuntime one")
	require.NoError(t, c.Create(t.Context(), desired.DeepCopy()), "the API server refused a Mutable document")

	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, c.Get(t.Context(), types.NamespacedName{Name: "bashible-0"}, stored))
	require.True(t, specUpToDate(stored, desired), "stored:   %+v\nrendered: %+v", stored.Spec, desired.Spec)
}

// A bashible node's document names only what the agent does there: a section it
// leaves alone is not sent at all, not even as {}.
func TestTheMutableRenderSendsNoEmptySections(t *testing.T) {
	raw, err := json.Marshal(renderMutableSpec(namedNode("worker-0"), "worker", clusterInputs{}))
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &spec))
	for _, section := range []string{"kernel", "network", "storage", "updatePolicy", "kubelet", "containerRuntime"} {
		require.NotContains(t, spec, section)
	}
}

// An object stored before the sections were left out still carries them as {}. They
// decode to the zero structs the new render has, so the object is up to date and no
// pass patches it.
func TestAStoredDocumentWithEmptySectionsIsUpToDate(t *testing.T) {
	desired := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
		Spec:       renderMutableSpec(namedNode("worker-0"), "worker", clusterInputs{}),
	}
	raw, err := json.Marshal(desired.Spec)
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(raw, &spec))
	spec["kernel"] = map[string]any{}
	spec["network"] = map[string]any{"dns": map[string]any{}, "ntp": map[string]any{}}
	spec["storage"] = map[string]any{}
	spec["updatePolicy"] = map[string]any{"window": map[string]any{}}
	asStored, err := json.Marshal(spec)
	require.NoError(t, err)

	stored := &internalv1alpha1.NodeConfig{ObjectMeta: desired.ObjectMeta}
	require.NoError(t, json.Unmarshal(asStored, &stored.Spec))
	require.True(t, specUpToDate(stored, desired))
}

func staticPodNames(pods []internalv1alpha1.StaticPod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	return names
}

// kubeSystemPodManifest is podManifest for a pod in kube-system, where the
// registry agent bashible writes runs.
func kubeSystemPodManifest(pod string) string {
	return strings.Replace(podManifest(pod), "namespace: d8-system", "namespace: kube-system", 1)
}

// mutableCluster holds what a bashible node's document is rendered from, and
// nothing an Engine node needs.
func mutableCluster(t *testing.T, digests string, objects ...client.Object) client.Client {
	t.Helper()

	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	base := []client.Object{
		apiServerEndpointSlice("192.168.199.10"),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: imagesDigestsConfigMapName},
			Data:       map[string]string{imagesDigestsKey: digests},
		},
		testenv.PlatformTemplatesConfigMap(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: registryPackagesProxyTokenSecret},
			Data:       map[string][]byte{registryPackagesProxyTokenKey: []byte("proxy-token")},
		},
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append(base, objects...)...).
		WithStatusSubresource(&deckhousev1alpha1.NodeExtensionRequest{}).
		Build()
}

// A release without the agent's package is broken, not an edition without the agent:
// the node's pass fails on it by name, and the document it has stays as it was.
func TestABashibleNodeKeepsItsDocumentWhileTheAgentPackageIsMissing(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	node := nodeInGroup("worker-0", "worker")
	existing := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Labels: map[string]string{
			nodecommon.NodeGroupLabel: "worker", managedByLabel: managedByValue,
		}},
		Spec: internalv1alpha1.NodeSpec{SystemType: internalv1alpha1.SystemTypeMutable, NodeName: "worker-0"},
	}
	digests := mutableDigests()
	delete(digests, "nodelet")
	cl := mutableCluster(t, digestsJSON(t, digests), ng, &node, existing)
	r := &Reconciler{}
	r.Client = cl
	r.sources = sourceReaderOver(cl)

	_, err := r.reconcileNodeObject(t.Context(), &node, logr.Discard(), newPass())
	require.ErrorContains(t, err, `"nodelet"`)

	stored := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: "worker-0"}, stored))
	require.Equal(t, existing.Spec, stored.Spec)
}

// namedNode is a node with a name and no labels.
func namedNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// A bashible node takes the platform's part of its document as the platform document has
// it, pods first and packages in order, then what the requests ask it for.
func TestTheMutableDocumentListsThePlatformThenTheRequests(t *testing.T) {
	in := clusterInputs{
		Platform: platformDocument{
			FeatureGates: map[string]bool{"extensions": true, "static-pods": true},
			Extensions: []internalv1alpha1.Extension{
				{Name: nodeletExtension, Digest: testNodeletPackageDigest, RequestedBy: platformExtensionRequestedBy},
				{Name: "jq", Digest: testenv.TestPlatformPackageDigests["jq"], RequestedBy: platformExtensionRequestedBy},
			},
			StaticPods: []internalv1alpha1.StaticPod{{Name: "early", Manifest: "kind: Pod"}},
		},
	}
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs([]deckhousev1alpha1.NodeExtensionRequest{
		ner("sds-utils", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension: deckhousev1alpha1.NodeExtension{Name: "sds-utils", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
		}),
	}, noPlatform)

	spec := renderMutableSpec(namedNode("worker-0"), "worker", in)

	require.Equal(t, []internalv1alpha1.Extension{
		in.Platform.Extensions[0],
		in.Platform.Extensions[1],
		{Name: "sds-utils", Digest: drbdDigest, RequestedBy: nerRequestedByPrefix + "sds-utils"},
	}, spec.Extensions)
	require.Equal(t, in.Platform.FeatureGates, spec.FeatureGates)
	require.Equal(t, in.Platform.StaticPods, spec.StaticPods)
}

// A requested pod named like a platform pod would make two entries under one list key,
// and the API would refuse the whole document: the platform's stays.
func TestAPlatformPodKeepsItsNameAgainstARequest(t *testing.T) {
	platform := []internalv1alpha1.StaticPod{{Name: "early", Manifest: "platform"}}
	requested := []internalv1alpha1.StaticPod{{Name: "early", Manifest: "request"}, {Name: "late", Manifest: "request"}}

	require.Equal(t, []internalv1alpha1.StaticPod{platform[0], requested[1]}, mergeStaticPods(platform, requested))
}

// A request narrowed by node labels reaches the bashible nodes that carry them, as it
// does on Engine.
func TestAPackageRequestSelectsABashibleNodeByItsLabels(t *testing.T) {
	var in clusterInputs
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs([]deckhousev1alpha1.NodeExtensionRequest{
		ner("sds-utils", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension:    deckhousev1alpha1.NodeExtension{Name: "sds-utils", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
			NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "drbd"}},
		}),
	}, noPlatform)

	labelled := renderMutableSpec(nodeWith(map[string]string{"storage": "drbd"}), "worker", in)
	bare := renderMutableSpec(nodeWith(nil), "worker", in)

	require.Equal(t, []string{"sds-utils"}, requestedNames(labelled.Extensions))
	require.Empty(t, requestedNames(bare.Extensions))
}

// A bashible node installs registry packages: a system extension goes to Engine nodes,
// and a package cannot load kernel modules, so neither request reaches it.
func TestABashibleNodeTakesNeitherSystemExtensionsNorKernelModules(t *testing.T) {
	var in clusterInputs
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs([]deckhousev1alpha1.NodeExtensionRequest{
		ner("sds-drbd", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension: deckhousev1alpha1.NodeExtension{Name: "drbd", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindSystemExtension},
		}),
		ner("gpu-tools", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension:     deckhousev1alpha1.NodeExtension{Name: "gpu-tools", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
			KernelModules: []deckhousev1alpha1.KernelModule{{Name: "nvidia"}},
		}),
	}, noPlatform)

	spec := renderMutableSpec(namedNode("worker-0"), "worker", in)

	require.Empty(t, requestedNames(spec.Extensions))
}

// A bashible node's inputs carry the requests settled for the whole cluster.
func TestReadMutableInputsSettlesTheRequests(t *testing.T) {
	request := ner("sds-utils", deckhousev1alpha1.NodeExtensionRequestSpec{
		Extension: deckhousev1alpha1.NodeExtension{Name: "sds-utils", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
	})
	cluster := mutableCluster(t, digestsJSON(t, mutableDigests()), &request)

	in, err := sourceReaderOver(cluster).readMutableInputs(t.Context())
	require.NoError(t, err)

	require.Len(t, in.NodeExtensions, 1)
}

// A pass delivers a package by the kind its requester declared: nothing is asked, so
// nothing is waited for and no retry is scheduled.
func TestAPassDeliversAPackageWithNobodyToAsk(t *testing.T) {
	ng := mutableGroup("worker", v1.SystemTypeMutable)
	node := nodeInGroup("worker-0", "worker")
	request := ner("sds-utils", deckhousev1alpha1.NodeExtensionRequestSpec{
		Extension: deckhousev1alpha1.NodeExtension{Name: "sds-utils", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
	})
	cl := mutableCluster(t, digestsJSON(t, mutableDigests()), ng, &node, &request)
	r := &Reconciler{}
	r.Client = cl
	r.sources = sourceReaderOver(cl)

	result, err := r.reconcileAllNodes(t.Context(), logr.Discard())

	require.NoError(t, err)
	require.Zero(t, result.RequeueAfter)
	config := &internalv1alpha1.NodeConfig{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: "worker-0"}, config))
	require.Equal(t, []string{"sds-utils"}, requestedNames(config.Spec.Extensions))
}

// Every node of a pass renders from the same memoised inputs: what one node's request
// adds must not land in another node's document.
func TestNodesOfOnePassDoNotShareTheirRequestedPackages(t *testing.T) {
	var in clusterInputs
	// A decoded platform document carries spare capacity behind its last element.
	in.Platform.Extensions = make([]internalv1alpha1.Extension, 1, 4)
	in.Platform.Extensions[0] = internalv1alpha1.Extension{Name: nodeletExtension, Digest: testNodeletPackageDigest, RequestedBy: platformExtensionRequestedBy}
	in.NodeExtensions, in.NodeExtensionConflicts = settleNERs([]deckhousev1alpha1.NodeExtensionRequest{
		ner("pkg-a", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension:    deckhousev1alpha1.NodeExtension{Name: "pkg-a", Digest: drbdDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
			NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "a"}},
		}),
		ner("pkg-b", deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension:    deckhousev1alpha1.NodeExtension{Name: "pkg-b", Digest: otherDigest, Kind: deckhousev1alpha1.ExtensionKindPackage},
			NodeSelector: deckhousev1alpha1.NodeSelector{MatchLabels: map[string]string{"storage": "b"}},
		}),
	}, in.Platform.Extensions)

	a := renderMutableSpec(nodeWith(map[string]string{"storage": "a"}), "worker", in)
	b := renderMutableSpec(nodeWith(map[string]string{"storage": "b"}), "worker", in)

	require.Equal(t, []string{"pkg-a"}, requestedNames(a.Extensions))
	require.Equal(t, []string{"pkg-b"}, requestedNames(b.Extensions))
	require.Len(t, in.Platform.Extensions, 1)
}
