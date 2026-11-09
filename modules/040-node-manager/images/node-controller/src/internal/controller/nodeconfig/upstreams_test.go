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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/registry/apis/upstreams"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var upstreamsEpoch = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

func registryUpstreamObject(name, match string, minutes int, upstream map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(registryUpstreamGVK)
	obj.SetName(name)
	obj.SetGeneration(1)
	obj.SetCreationTimestamp(metav1.NewTime(upstreamsEpoch.Add(time.Duration(minutes) * time.Minute)))
	obj.Object["spec"] = map[string]any{"match": match, "upstream": upstream}
	return obj
}

func moduleSourceObject(name, repo, ca string, minutes int) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(moduleSourceGVK)
	obj.SetName(name)
	obj.SetCreationTimestamp(metav1.NewTime(upstreamsEpoch.Add(time.Duration(minutes) * time.Minute)))
	obj.Object["spec"] = map[string]any{"registry": map[string]any{"repo": repo, "scheme": "HTTPS", "ca": ca}}
	return obj
}

func upstreamsCluster(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	base := []client.Object{&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: d8SystemNS, Name: deckhouseRegistrySecret},
		Data:       map[string][]byte{registryAddressKey: []byte(testenv.TestRegistryAddress)},
	}}
	statusOf := &unstructured.Unstructured{}
	statusOf.SetGroupVersionKind(registryUpstreamGVK)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(base, objects...)...).
		WithStatusSubresource(statusOf).Build()
}

func matches(routes []internalv1alpha1.RegistryUpstream) []string {
	out := make([]string, 0, len(routes))
	for _, route := range routes {
		out = append(out, route.RequestedBy.Kind+"/"+route.RequestedBy.Name+"="+route.Match)
	}
	return out
}

// testCertificate is a self-signed certificate made for the test run, PEM-encoded: the
// arbitration accepts a CA that parses and nothing else.
func testCertificate(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "vendor.example.com"},
		NotBefore:    upstreamsEpoch,
		NotAfter:     upstreamsEpoch.Add(24 * time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func primaryOf(address string) *registryv1alpha1.Upstream {
	return &registryv1alpha1.Upstream{Endpoint: registryv1alpha1.Endpoint{Host: address}}
}

func upstreamOf(t *testing.T, name, match string, minutes int, upstream map[string]any) registryv1alpha1.RegistryUpstream {
	t.Helper()
	if upstream == nil {
		upstream = map[string]any{"host": "vendor.example.com"}
	}
	obj, err := toRegistryUpstream(registryUpstreamObject(name, match, minutes, upstream))
	require.NoError(t, err)
	return obj
}

// User story: as an operator with the registry agent off, I want my RegistryUpstream objects
// and my module sources' authorities on every node, settled by the rules the registry module
// settles them by, so a node pulls what the objects say whoever arbitrates.
func TestArbitrateRoutes(t *testing.T) {
	early, late := testCertificate(t), testCertificate(t)
	routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), []registryv1alpha1.RegistryUpstream{
		upstreamOf(t, "newer", "images.example.com", 5, nil),
		upstreamOf(t, "older", "images.example.com", 0, nil),
		upstreamOf(t, "shadow", testenv.TestRegistryAddress, 0, nil),
		upstreamOf(t, "in-cluster", "registry.d8-system.svc:5001", 0, nil),
	}, []moduleSourceCA{
		{Name: "partner-late", Created: upstreamsEpoch.Add(time.Hour), Host: "modules.example.com", Scheme: "HTTPS", CA: late},
		{Name: "partner", Created: upstreamsEpoch, Host: "modules.example.com", Scheme: "HTTPS", CA: early},
		{Name: "own", Created: upstreamsEpoch, Host: testenv.TestRegistryAddress, Scheme: "HTTPS", CA: "PRIMARY"},
	})

	require.Equal(t, []string{"RegistryUpstream/older=images.example.com", "ModuleSource/partner=modules.example.com"}, matches(routes.Routes))
	require.Equal(t, early, routes.Routes[1].Upstream.CA, "the older source keeps a host")
	require.True(t, routes.Verdicts["older"].Accepted)
	for _, refused := range []string{"newer", "shadow", "in-cluster"} {
		require.False(t, routes.Verdicts[refused].Accepted, refused)
	}
	require.Empty(t, routes.Dropped)
}

// A ModuleSource reserves nothing: the verdict is registry-controller's, which knows no
// ModuleSource. An accepted object routing a source's registry takes it over on the nodes,
// whatever the case of either, and the source's authority gives way.
func TestARegistryUpstreamTakesAModuleSourceRegistryOver(t *testing.T) {
	sourceCA := testCertificate(t)
	routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), []registryv1alpha1.RegistryUpstream{
		upstreamOf(t, "over-modules", "Modules.Example.com", 10, nil),
	}, []moduleSourceCA{
		{Name: "partner", Created: upstreamsEpoch, Host: "modules.example.com", Scheme: "HTTPS", CA: sourceCA},
	})

	require.True(t, routes.Verdicts["over-modules"].Accepted)
	require.Equal(t, []string{"RegistryUpstream/over-modules=modules.example.com"}, matches(routes.Routes))
	require.Contains(t, routes.Dropped["partner"], "RegistryUpstream over-modules routes modules.example.com")
}

// A route's match names a directory on the node, so it is written lower-case: two spellings
// of one registry are one directory, and nodelet compares them so.
func TestARouteMatchIsLowerCase(t *testing.T) {
	sourceCA := testCertificate(t)
	routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), []registryv1alpha1.RegistryUpstream{
		upstreamOf(t, "ghcr", "GHCR.io", 0, nil),
		upstreamOf(t, "ghcr-again", "ghcr.IO", 1, nil),
	}, []moduleSourceCA{{Name: "partner", Created: upstreamsEpoch, Host: "Modules.Example.com", Scheme: "HTTPS", CA: sourceCA}})

	require.Equal(t, []string{"RegistryUpstream/ghcr=ghcr.io", "ModuleSource/partner=modules.example.com"}, matches(routes.Routes))
	require.False(t, routes.Verdicts["ghcr-again"].Accepted)
}

// The arbitration reserves the mirrors of the module's primary too, as registry-controller
// does from the same configuration: the verdict must not depend on who reaches it.
func TestTheArbitrationReservesThePrimaryMirrors(t *testing.T) {
	config := &unstructured.Unstructured{}
	config.SetAPIVersion(registryConfigAPIVersion)
	config.SetKind(registryConfigKind)
	config.SetName(registryConfigName)
	config.Object["spec"] = map[string]any{"primary": map[string]any{"upstream": map[string]any{
		"host": "registry.deckhouse.io", "mirrors": []any{map[string]any{"host": "Mirror.Example.com"}},
	}}}
	cluster := upstreamsCluster(t, config, registryUpstreamObject("hijack", "mirror.example.com", 0, map[string]any{"host": "vendor.example.com"}))

	primary, _, err := sourceReaderOver(cluster).readRegistry(t.Context())
	require.NoError(t, err)
	routes, err := sourceReaderOver(cluster).readRegistryRoutes(t.Context(), primary)
	require.NoError(t, err)
	require.Empty(t, routes.Routes)
	require.Equal(t, registryv1alpha1.ReasonShadowsPrimary, routes.Verdicts["hijack"].Reason)
}

// The ModuleSource routes come after the objects and share what is left of the budget the
// objects were held to: past it the NodeConfig of every node would be refused.
func TestModuleSourcesShareTheBudget(t *testing.T) {
	t.Run("the count", func(t *testing.T) {
		var objects []registryv1alpha1.RegistryUpstream
		for i := range upstreams.MaxRoutes {
			objects = append(objects, upstreamOf(t, fmt.Sprintf("u%03d", i), fmt.Sprintf("r%03d.example.com", i), i, nil))
		}
		routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), objects,
			[]moduleSourceCA{{Name: "partner", Created: upstreamsEpoch, Host: "modules.example.com", Scheme: "HTTPS", CA: testCertificate(t)}})

		require.Len(t, routes.Routes, upstreams.MaxRoutes)
		require.Contains(t, routes.Dropped["partner"], "do not fit")
	})

	t.Run("the size", func(t *testing.T) {
		ca := testCertificate(t)
		bundle := strings.Repeat(ca, 60*1024/len(ca))
		var objects []registryv1alpha1.RegistryUpstream
		for i := range 2 {
			objects = append(objects, upstreamOf(t, fmt.Sprintf("u%d", i), fmt.Sprintf("r%d.example.com", i), i, map[string]any{
				"host": "vendor.example.com", "ca": bundle,
				"mirrors": []any{map[string]any{"host": "mirror.example.com", "ca": bundle}},
			}))
		}
		routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), objects, []moduleSourceCA{
			{Name: "large", Created: upstreamsEpoch, Host: "large.example.com", Scheme: "HTTPS", CA: bundle},
			{Name: "small", Created: upstreamsEpoch.Add(time.Minute), Host: "small.example.com", Scheme: "HTTPS", CA: ca},
		})

		require.True(t, routes.Verdicts["u0"].Accepted)
		require.True(t, routes.Verdicts["u1"].Accepted)
		require.Contains(t, routes.Dropped["large"], "do not fit")
		require.Equal(t, []string{"RegistryUpstream/u0=r0.example.com", "RegistryUpstream/u1=r1.example.com", "ModuleSource/small=small.example.com"}, matches(routes.Routes))
	})
}

// An entry is the object's spec, field for field, whichever of the two credential forms it uses.
func TestARouteCarriesTheObjectsSpec(t *testing.T) {
	ca := testCertificate(t)
	obj := registryUpstreamObject("virtualization", "images.example.com", 0, map[string]any{
		"scheme": "HTTPS", "host": "vendor.example.com", "path": "/virtualization", "ca": ca,
		"auth":    map[string]any{"username": "robot", "password": "dh-test-secret-0123456789abcdef"},
		"mirrors": []any{map[string]any{"scheme": "HTTP", "host": "mirror.example.com:5000", "auth": map[string]any{"auth": "dXNlcm5hbWU6cGFzc3dvcmQ="}}},
	})
	upstream, err := toRegistryUpstream(obj)
	require.NoError(t, err)

	routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), []registryv1alpha1.RegistryUpstream{upstream}, nil)
	require.Equal(t, []internalv1alpha1.RegistryUpstream{{
		RequestedBy: internalv1alpha1.ObjectOwner{Kind: internalv1alpha1.ObjectOwnerKindRegistryUpstream, Name: "virtualization"},
		Match:       "images.example.com",
		Upstream: internalv1alpha1.RegistryUpstreamEndpoints{
			RegistryEndpoint: internalv1alpha1.RegistryEndpoint{
				Scheme: "HTTPS", Host: "vendor.example.com", Path: "/virtualization", CA: ca,
				Auth: &internalv1alpha1.RegistryEndpointAuth{Username: "robot", Password: "dh-test-secret-0123456789abcdef"},
			},
			Mirrors: []internalv1alpha1.RegistryEndpoint{{
				Scheme: "HTTP", Host: "mirror.example.com:5000",
				Auth: &internalv1alpha1.RegistryEndpointAuth{Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="},
			}},
		},
	}}, routes.Routes)
}

// On an Engine node the routes are the agent's to apply while it owns registry.d. A bashible
// node gets them whoever owns it — nodelet sees the agent's _default there and steps aside —
// and no ModuleSource route: bashible writes those authorities there itself. Of the primary
// it gets the address alone: nodelet keeps the routes off it, and needs nothing else of it.
func TestTheRoutesFollowWhoOwnsRegistryD(t *testing.T) {
	in := clusterInputs{
		Registry: &internalv1alpha1.Registry{
			Address: testenv.TestRegistryAddress, Path: "/deckhouse/ee", Scheme: "HTTPS", CABundle: "PRIMARY-CA", Auth: []byte("robot:dh-test-secret-0123456789abcdef"),
		},
		RegistryAgentMode: true,
		RegistryRoutes: registryRoutes{Routes: []internalv1alpha1.RegistryUpstream{
			{RequestedBy: internalv1alpha1.ObjectOwner{Kind: internalv1alpha1.ObjectOwnerKindRegistryUpstream, Name: "virt"}, Match: "images.example.com"},
			{RequestedBy: internalv1alpha1.ObjectOwner{Kind: internalv1alpha1.ObjectOwnerKindModuleSource, Name: "partner"}, Match: "modules.example.com"},
		}},
	}

	require.Equal(t, []string{"RegistryUpstream/virt=images.example.com", "ModuleSource/partner=modules.example.com"}, matches(nodeRegistry(in, false).Upstreams))
	require.Empty(t, nodeRegistry(in, true).Upstreams)
	require.Equal(t, *in.Registry, *nodeRegistry(in, true), "an Engine node keeps the whole primary")
	require.Nil(t, nodeRegistry(clusterInputs{}, false))

	mutable := mutableNodeRegistry(in)
	require.Equal(t, []string{"RegistryUpstream/virt=images.example.com"}, matches(mutable.Upstreams), "in agent mode too")
	require.Equal(t, internalv1alpha1.Registry{Address: testenv.TestRegistryAddress, Upstreams: mutable.Upstreams}, *mutable,
		"no path, scheme, authority or credentials on a bashible node")
	require.Nil(t, mutableNodeRegistry(clusterInputs{}))
	require.Nil(t, in.Registry.Upstreams, "the shared input is never written through")
}

// A source without an authority, or without a repository, has nothing for a node.
func TestAModuleSourceRoutesOnlyItsAuthority(t *testing.T) {
	sourceCA := testCertificate(t)
	routes, err := sourceReaderOver(upstreamsCluster(t,
		moduleSourceObject("with-ca", "Modules.Example.com/deckhouse/modules", sourceCA, 0),
		moduleSourceObject("without-ca", "other.example.com/modules", "", 0),
	)).readRegistryRoutes(t.Context(), &internalv1alpha1.Registry{Address: testenv.TestRegistryAddress})
	require.NoError(t, err)
	require.Equal(t, []string{"ModuleSource/with-ca=modules.example.com"}, matches(routes.Routes))
	require.Equal(t, internalv1alpha1.RegistryEndpoint{Scheme: "HTTPS", Host: "modules.example.com", CA: sourceCA}, routes.Routes[0].Upstream.RegistryEndpoint)
}

// A ModuleSource's repository and authority are checked by nothing before they reach this
// list, and one the NodeConfig schema refuses would cost every Engine node its document:
// such a source is dropped, and logged.
func TestAModuleSourceTheNodesWouldRefuseIsDropped(t *testing.T) {
	sourceCA := testCertificate(t)
	routes := arbitrateRoutes(primaryOf(testenv.TestRegistryAddress), nil, []moduleSourceCA{
		{Name: "ipv6", Created: upstreamsEpoch, Host: "[fd00::1]:5000", Scheme: "HTTPS", CA: sourceCA},
		{Name: "url", Created: upstreamsEpoch, Host: "https:", Scheme: "HTTPS", CA: sourceCA},
		{Name: "port", Created: upstreamsEpoch, Host: "reg.example.com:99999", Scheme: "HTTPS", CA: sourceCA},
		{Name: "not-pem", Created: upstreamsEpoch, Host: "pem.example.com", Scheme: "HTTPS", CA: "SOURCE-CA"},
		{Name: "large", Created: upstreamsEpoch, Host: "large.example.com", Scheme: "HTTPS", CA: strings.Repeat(sourceCA, upstreams.MaxCALength/len(sourceCA)+1)},
		{Name: "scheme", Created: upstreamsEpoch, Host: "scheme.example.com", Scheme: "FTP", CA: sourceCA},
		{Name: "fine", Created: upstreamsEpoch, Host: "fine.example.com", Scheme: "HTTPS", CA: sourceCA},
	})

	require.Equal(t, []string{"ModuleSource/fine=fine.example.com"}, matches(routes.Routes))
	for name, want := range map[string]string{
		"ipv6": "is not a registry host", "url": "is not a registry host", "port": "out of range",
		"not-pem": "not a PEM bundle", "large": "longer than 65536", "scheme": "neither HTTP nor HTTPS",
	} {
		require.Contains(t, routes.Dropped[name], want, name)
	}
}

// While no agent runs, the verdict lands on the object the way the registry module writes it,
// and a verdict that turns clears what the last one said.
func TestTheVerdictLandsOnTheObject(t *testing.T) {
	cluster := upstreamsCluster(t,
		registryUpstreamObject("older", "images.example.com", 0, map[string]any{"host": "vendor.example.com"}),
		registryUpstreamObject("newer", "images.example.com", 5, map[string]any{"host": "vendor.example.com"}),
	)
	r := &Reconciler{Base: register.Base{Client: cluster}}
	require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))

	status := func(name string) map[string]any {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(registryUpstreamGVK)
		require.NoError(t, cluster.Get(t.Context(), types.NamespacedName{Name: name}, obj))
		status, _, _ := unstructured.NestedMap(obj.Object, "status")
		return status
	}
	require.Equal(t, true, status("older")["accepted"])
	require.Equal(t, false, status("newer")["accepted"])
	require.Contains(t, status("newer")["conflict"], "RegistryUpstream/older")

	older := &unstructured.Unstructured{}
	older.SetGroupVersionKind(registryUpstreamGVK)
	require.NoError(t, cluster.Get(t.Context(), types.NamespacedName{Name: "older"}, older))
	require.NoError(t, cluster.Delete(t.Context(), older))
	require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))
	require.Equal(t, true, status("newer")["accepted"])
	require.NotContains(t, status("newer"), "conflict", "an accepted object carries no conflict")
}

// The manager's cache holds no deckhouse-registry (common.CacheOptions scopes d8-system's
// Secrets to registry-bashible-config), so the verdicts are arbitrated from the live reads the
// renders use — pipeline 894605 found every status unwritten when they came from the cache.
func TestTheVerdictsAreReadLiveNotFromTheCache(t *testing.T) {
	upstream := registryUpstreamObject("transit", "images.example.com", 0, map[string]any{"host": "vendor.example.com"})
	live := upstreamsCluster(t, upstream.DeepCopy())

	scheme := k8sruntime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	statusOf := &unstructured.Unstructured{}
	statusOf.SetGroupVersionKind(registryUpstreamGVK)
	cached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(upstream).WithStatusSubresource(statusOf).Build()

	r := &Reconciler{Base: register.Base{Client: cached}, sources: sourceReaderOver(live)}
	require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(registryUpstreamGVK)
	require.NoError(t, cached.Get(t.Context(), types.NamespacedName{Name: "transit"}, obj))
	accepted, _, _ := unstructured.NestedBool(obj.Object, "status", "accepted")
	require.True(t, accepted)
}

func acceptedCondition(t *testing.T, cluster client.Client, name string) map[string]any {
	t.Helper()
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(registryUpstreamGVK)
	require.NoError(t, cluster.Get(t.Context(), types.NamespacedName{Name: name}, obj))
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, condition := range conditions {
		if fields := condition.(map[string]any); fields["type"] == registryv1alpha1.ConditionAccepted {
			return fields
		}
	}
	t.Fatalf("RegistryUpstream %s has no Accepted condition", name)
	return nil
}

func nodeGroupWithCRI(name string, systemType v1.SystemType, cri v1.CRIType) *v1.NodeGroup {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: name}}
	ng.Spec.NodeType = v1.NodeTypeStatic
	ng.Spec.SystemType = systemType
	if cri != "" {
		ng.Spec.CRI = &v1.CRISpec{Type: cri}
	}
	return ng
}

// User story: as an operator with the registry module disabled and some groups on
// containerd v1, I want to see on the object that those nodes do not apply it, rather than
// find out from a pull that went to the wrong registry. The object stays accepted: every
// other node applies it, and registry-controller, which knows no nodes, would say the same.
func TestTheStatusTellsWhatContainerdV1NodesDoNotApply(t *testing.T) {
	objects := func() []client.Object {
		return []client.Object{
			registryUpstreamObject("with-path", "images.example.com", 0, map[string]any{"host": "vendor.example.com", "path": "/virt"}),
			registryUpstreamObject("plain", "ghcr.example.com", 1, map[string]any{"host": "proxy.example.com"}),
			registryUpstreamObject("same-registry", "quay.example.com", 2, map[string]any{"host": "Proxy.Example.com"}),
			registryUpstreamObject("over-primary", "docker.example.com", 3, map[string]any{"host": testenv.TestRegistryAddress}),
			registryUpstreamObject("both-schemes", "quay2.example.com", 4, map[string]any{
				"host": "twice.example.com", "mirrors": []any{map[string]any{"scheme": "HTTP", "host": "Twice.Example.com"}},
			}),
		}
	}
	message := func(cluster client.Client, name string) string {
		condition := acceptedCondition(t, cluster, name)
		require.Equal(t, "True", condition["status"], name)
		return condition["message"].(string)
	}

	t.Run("module disabled, a containerd v1 group", func(t *testing.T) {
		cluster := upstreamsCluster(t, append(objects(),
			nodeGroupWithCRI("legacy", v1.SystemTypeLegacyMutable, v1.CRITypeContainerd),
			nodeGroupWithCRI("modern", v1.SystemTypeLegacyMutable, v1.CRITypeContainerdV2),
		)...)
		r := &Reconciler{Base: register.Base{Client: cluster}}
		require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))

		require.Contains(t, message(cluster, "with-path"), "Not applied on the containerd v1 nodes of NodeGroup legacy")
		require.Contains(t, message(cluster, "with-path"), "repository path")
		require.Equal(t, "The upstream is routed on every node.", message(cluster, "plain"))
		require.Contains(t, message(cluster, "same-registry"), `registry "proxy.example.com" is configured there already`)
		require.Contains(t, message(cluster, "over-primary"), fmt.Sprintf("registry %q is configured there already", testenv.TestRegistryAddress))
		require.Contains(t, message(cluster, "both-schemes"), `registry "twice.example.com" is configured there already`)
	})

	t.Run("the cluster default is containerd v1", func(t *testing.T) {
		cluster := upstreamsCluster(t, append(objects(),
			nodeGroupWithCRI("default", v1.SystemTypeLegacyMutable, ""),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: clusterConfigSecretName},
				Data:       map[string][]byte{clusterConfigKey: []byte("defaultCRI: Containerd\n")},
			},
		)...)
		r := &Reconciler{Base: register.Base{Client: cluster}}
		require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))
		require.Contains(t, message(cluster, "with-path"), "NodeGroup default")
	})

	for name, extra := range map[string][]client.Object{
		"no containerd v1 group": {
			nodeGroupWithCRI("modern", v1.SystemTypeLegacyMutable, v1.CRITypeContainerdV2),
			nodeGroupWithCRI("engine", v1.SystemTypeLegacyImmutable, v1.CRITypeContainerd),
		},
		"the module configures the nodes": {
			nodeGroupWithCRI("legacy", v1.SystemTypeLegacyMutable, v1.CRITypeContainerd),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: d8SystemNS, Name: registryBashibleConfigSecret},
				Data:       map[string][]byte{registryBashibleConfigKey: []byte("mode: Unmanaged\n")},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cluster := upstreamsCluster(t, append(objects(), extra...)...)
			r := &Reconciler{Base: register.Base{Client: cluster}}
			require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))
			require.Equal(t, "The upstream is routed on every node.", message(cluster, "with-path"))
		})
	}
}

// User story: as an operator whose RegistryUpstream routes the registry of a ModuleSource,
// I want to see on the object that the nodes bashible configures keep that registry as the
// ModuleSource configures it: step 030 writes its directory there, and nodelet leaves it be.
func TestTheStatusTellsWhereAModuleSourceKeepsItsRegistry(t *testing.T) {
	sourceCA := testCertificate(t)
	objects := func() []client.Object {
		return []client.Object{
			registryUpstreamObject("over-modules", "Modules.Example.com", 0, map[string]any{"host": "vendor.example.com"}),
			registryUpstreamObject("plain", "ghcr.example.com", 1, map[string]any{"host": "proxy.example.com"}),
			moduleSourceObject("partner", "modules.example.com/modules", sourceCA, 0),
			nodeGroupWithCRI("legacy", v1.SystemTypeLegacyMutable, v1.CRITypeContainerd),
			nodeGroupWithCRI("modern", v1.SystemTypeLegacyMutable, v1.CRITypeContainerdV2),
			nodeGroupWithCRI("engine", v1.SystemTypeLegacyImmutable, v1.CRITypeContainerdV2),
		}
	}
	message := func(cluster client.Client, name string) string {
		condition := acceptedCondition(t, cluster, name)
		require.Equal(t, "True", condition["status"], name)
		return condition["message"].(string)
	}

	t.Run("module disabled: containerd v1 reads config.toml, where the object wins", func(t *testing.T) {
		cluster := upstreamsCluster(t, objects()...)
		r := &Reconciler{Base: register.Base{Client: cluster}}
		require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))
		require.Equal(t, "The upstream is routed. Not applied on the nodes of NodeGroup modern, which bashible configures: "+
			`ModuleSource partner configures registry "modules.example.com" there.`, message(cluster, "over-modules"))
		require.Equal(t, "The upstream is routed on every node.", message(cluster, "plain"))
	})

	t.Run("the module configures the nodes: every runtime reads registry.d", func(t *testing.T) {
		cluster := upstreamsCluster(t, append(objects(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: d8SystemNS, Name: registryBashibleConfigSecret},
			Data:       map[string][]byte{registryBashibleConfigKey: []byte("mode: Direct\n")},
		})...)
		r := &Reconciler{Base: register.Base{Client: cluster}}
		require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))
		require.Contains(t, message(cluster, "over-modules"), "NodeGroup legacy, modern, which bashible configures")
	})
}

// One object nobody can read must not cost every node its document: it is left out, and
// gets no status, while every other object is routed and told so.
func TestAnUnreadableObjectIsLeftOut(t *testing.T) {
	broken := registryUpstreamObject("broken", "images.example.com", 0, map[string]any{"host": "vendor.example.com"})
	broken.Object["spec"].(map[string]any)["match"] = int64(42)
	sourceCA := testCertificate(t)
	brokenSource := moduleSourceObject("broken-source", "other.example.com/modules", sourceCA, 0)
	brokenSource.Object["spec"].(map[string]any)["registry"].(map[string]any)["ca"] = int64(42)
	cluster := upstreamsCluster(t,
		broken,
		brokenSource,
		registryUpstreamObject("fine", "ghcr.example.com", 1, map[string]any{"host": "proxy.example.com"}),
		moduleSourceObject("partner", "modules.example.com/modules", sourceCA, 0),
	)

	routes, err := sourceReaderOver(cluster).readRegistryRoutes(t.Context(), &internalv1alpha1.Registry{Address: testenv.TestRegistryAddress})
	require.NoError(t, err)
	require.Equal(t, []string{"RegistryUpstream/fine=ghcr.example.com", "ModuleSource/partner=modules.example.com"}, matches(routes.Routes))
	require.NotContains(t, routes.Verdicts, "broken")

	r := &Reconciler{Base: register.Base{Client: cluster}}
	require.NoError(t, r.reconcileRegistryUpstreamStatuses(t.Context(), logr.Discard()))
	require.Equal(t, "True", acceptedCondition(t, cluster, "fine")["status"])
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(registryUpstreamGVK)
	require.NoError(t, cluster.Get(t.Context(), types.NamespacedName{Name: "broken"}, obj))
	require.NotContains(t, obj.Object, "status")
}

// The arbitration holds every list it hands out within its limits, and the shipped
// NodeConfig schema (nodelet's) enforces the same ones: a route past the schema would
// make the API server refuse the document of every node, a schema past the arbitration
// would let nothing through that the arbitration refuses anyway.
func TestTheNodeConfigLimitsAreTheArbitrations(t *testing.T) {
	list := crdField(t, nodeConfigCRDSchema(t), "spec", "registry", "upstreams")
	require.Equal(t, float64(upstreams.MaxRoutes), list["maxItems"], "upstreams")

	itemsOf := func(field map[string]any) map[string]any {
		t.Helper()
		items, ok := field["items"].(map[string]any)
		require.True(t, ok, "the field has no items")
		return items
	}
	route := itemsOf(list)
	require.Equal(t, float64(upstreams.MaxMatchLength), crdField(t, route, "match")["maxLength"], "match")

	upstream := crdField(t, route, "upstream")
	mirrors := crdField(t, upstream, "mirrors")
	require.Equal(t, float64(upstreams.MaxMirrors), mirrors["maxItems"], "mirrors")

	for name, endpoint := range map[string]map[string]any{"upstream": upstream, "mirror": itemsOf(mirrors)} {
		for field, limit := range map[string]int{
			"host": upstreams.MaxHostLength,
			"path": upstreams.MaxPathLength,
			"ca":   upstreams.MaxCALength,
		} {
			require.Equal(t, float64(limit), crdField(t, endpoint, field)["maxLength"], "%s.%s", name, field)
		}
		for field, limit := range map[string]int{
			"username": upstreams.MaxUsernameLength,
			"password": upstreams.MaxPasswordLength,
			"auth":     upstreams.MaxAuthLength,
		} {
			require.Equal(t, float64(limit), crdField(t, endpoint, "auth", field)["maxLength"], "%s.auth.%s", name, field)
		}
	}
}
