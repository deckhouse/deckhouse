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

package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/bashiblecontext"
	"github.com/deckhouse/deckhouse/pkg/log"
	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
)

// The goldens are control-plane-manager's, not a copy: this test is the offline stand-in for the
// step-8 comparison, which diffs the Secret the tenant writes against the one the parent wrote.
// context.golden.yaml is frozen — the writer that produced it is gone and these bytes are what
// the fleet was bootstrapped with. external-inputs.golden.yaml still has a generator; regenerate
// it with
//
//	go test ./internal/controllers/virtual-control-plane-configuration/bashible-apiserver/ -update-goldens
//
// in modules/040-control-plane-manager/images/control-plane-manager/src.
const cpmGoldenDir = "../../040-control-plane-manager/images/control-plane-manager/src/internal/controllers/virtual-control-plane-configuration/bashible-apiserver/testdata"

// The tenant cluster the golden facts describe. Everything here is what a virtual control plane
// actually creates in its tenant, and every value deliberately disagrees with the corresponding
// golden field so the test proves the inputs win rather than coinciding with them.
const (
	tenantClusterUUID      = "99999999-8888-7777-6666-555555555555"
	tenantALBVIP           = "10.20.30.40"
	tenantDeckhouseChannel = "stable"
	tenantDeckhouseVersion = "v1.72.3"
	tenantDeckhouseEdition = "EE"
)

func tenantScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, discoveryv1.AddToScheme(scheme))
	return scheme
}

// tenantObjects is what a VCP tenant holds when bashible-apiserver-context is due: the
// ClusterConfiguration and cluster UUID control-plane-manager seeded, the version info the
// tenant's own Deckhouse published, and the default/kubernetes EndpointSlice pointing at the ALB.
// Absent on purpose: kube-apiserver Pods, a kube-dns Service, the bootstrap-token Secrets, the
// registry-packages-proxy token and kubernetes-api-proxy-discovery-cert — none of them exist in a
// tenant, which is why those fields have to be published.
func tenantObjects() []runtime.Object {
	clusterConfiguration := `apiVersion: deckhouse.io/v1
clusterDomain: cluster.virtual
clusterType: Static
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.31"
podSubnetCIDR: 10.244.0.0/16
serviceSubnetCIDR: 10.96.0.0/12
`

	return []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
			Data:       map[string][]byte{"cluster-configuration.yaml": []byte(clusterConfiguration)},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-uuid", Namespace: "kube-system"},
			Data:       map[string]string{"cluster-uuid": tenantClusterUUID},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "d8-deckhouse-version-info", Namespace: "d8-system"},
			Data: map[string]string{"data.json": `{"channel":"` + tenantDeckhouseChannel +
				`","version":"` + tenantDeckhouseVersion + `","edition":"` + tenantDeckhouseEdition + `"}`},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta:  metav1.ObjectMeta{Name: "kubernetes", Namespace: "default"},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{tenantALBVIP}}},
			Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("https"), Port: ptr.To(int32(6443))}},
		},
	}
}

func tenantBashibleContextService(t *testing.T) *bashiblecontext.Service {
	t.Helper()

	return &bashiblecontext.Service{
		Client: fake.NewClientBuilder().
			WithScheme(tenantScheme(t)).
			WithRuntimeObjects(tenantObjects()...).
			Build(),
		// A nested Deckhouse Pod runs with automountServiceAccountToken: false in the parent
		// cluster; point at a path that cannot exist so the test never picks up a CA from the
		// machine running it.
		RootCAFile: filepath.Join(t.TempDir(), "no-such-ca.crt"),
	}
}

func readCPMGolden(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(cpmGoldenDir, name))
	require.NoError(t, err)
	return data
}

func asDocument(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()

	doc := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	return doc
}

// TestBashibleContextVCPMatchesControlPlaneManager is the acceptance gate of step 8 in offline
// form: the same facts, assembled from the other direction, must serialise to the same bytes.
// Byte equality also covers key ordering, which is only equal because sigs.k8s.io/yaml sorts
// alphabetically on both sides — a struct on control-plane-manager's, a map on this one.
func TestBashibleContextVCPMatchesControlPlaneManager(t *testing.T) {
	inputs, reason := usableBashibleExternalInputs(string(readCPMGolden(t, "external-inputs.golden.yaml")))
	require.Empty(t, reason)
	require.NotNil(t, inputs)

	service := tenantBashibleContextService(t)
	assembled, err := service.Build(context.Background(), service.ReadGlobals(context.Background()), nil)
	require.NoError(t, err)

	applyBashibleExternalInputs(assembled, inputs)

	got, err := bashiblecontext.Marshal(assembled)
	require.NoError(t, err)
	require.Equal(t, string(readCPMGolden(t, "context.golden.yaml")), string(got))
}

// TestBashibleContextVCPTenantComputedDivergences pins what the tenant would publish on its own,
// so the reason every field is in the contract stays visible. clusterDomain is the only one the
// tenant gets right unaided; the rest describe either the tenant instead of the virtual control
// plane, or nothing at all.
func TestBashibleContextVCPTenantComputedDivergences(t *testing.T) {
	service := tenantBashibleContextService(t)
	assembled, err := service.Build(context.Background(), service.ReadGlobals(context.Background()), nil)
	require.NoError(t, err)

	raw, err := bashiblecontext.Marshal(assembled)
	require.NoError(t, err)

	tenant := asDocument(t, raw)
	parent := asDocument(t, readCPMGolden(t, "context.golden.yaml"))

	var differ []string
	for key, want := range parent {
		if got, ok := tenant[key]; !ok || !equalYAML(t, got, want) {
			differ = append(differ, key)
		}
	}
	sort.Strings(differ)

	require.Equal(t, []string{
		"allowedBundles",          // the library lists all eight distributions, the VCP allows one
		"apiserverEndpoints",      // discovered from default/kubernetes: the ALB address, not the ALB hostname
		"apiserverProxyCerts",     // kubernetes-api-proxy-discovery-cert is not created in a tenant
		"bootstrapTokens",         // the VCP join token carries no node-manager.deckhouse.io/node-group label
		"clusterDNSAddress",       // kube-dns has not converged yet; once it has, its Service carries the same address
		"clusterMasterEndpoints",  // derived from the discovered endpoint, with in-cluster RPP ports
		"clusterUUID",             // d8-cluster-uuid holds the tenant UID, the context wants the parent's
		"deckhouse",               // d8-deckhouse-version-info holds the tenant's real channel/version/edition
		"kubernetesCA",            // no projected serviceaccount CA in the nested Pod
		"nodeGroups",              // a tenant has no NodeGroup objects at all
		"packagesProxy",           // no registry-packages-proxy token Secret, and direct is a VCP fact
		"podSubnetNodeCIDRPrefix", // absent from the ClusterConfiguration the VCP writes
	}, differ)

	require.Equal(t, "cluster.virtual", tenant["clusterDomain"])
	require.Equal(t, map[string]interface{}{
		"channel": tenantDeckhouseChannel,
		"version": tenantDeckhouseVersion,
		"edition": tenantDeckhouseEdition,
	}, tenant["deckhouse"])
	require.Equal(t, tenantClusterUUID, tenant["clusterUUID"])
	require.Equal(t, "", tenant["podSubnetNodeCIDRPrefix"])
	require.Equal(t, "", tenant["clusterDNSAddress"])
	require.NotContains(t, tenant, "kubernetesCA")
	require.NotContains(t, tenant, "apiserverProxyCerts")
	require.Equal(t, []interface{}{tenantALBVIP + ":6443"}, tenant["apiserverEndpoints"])

	// Nothing the tenant cannot see leaks in either: no cloud provider, no proxy and no
	// control-plane arguments Secret exist there, and the context golden has none of those keys.
	for _, key := range []string{"cloudProvider", "proxy", "nodeStatusUpdateFrequency", "allowedKubeletFeatureGates"} {
		require.NotContains(t, tenant, key)
		require.NotContains(t, parent, key)
	}
}

// TestBashibleContextVCPControlPlaneArgumentsLeak pins the one divergence the inputs cannot
// close, because it is an extra key rather than a different value. control-plane-manager enables
// itself wherever global.clusterConfiguration exists, which includes a tenant, and its
// arguments Secret makes the library add keys the context written by the parent has never had.
// If the step-8 diff comes back with these two lines, this is why.
func TestBashibleContextVCPControlPlaneArgumentsLeak(t *testing.T) {
	objects := append(tenantObjects(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-control-plane-manager-control-plane-arguments", Namespace: "kube-system"},
		Data: map[string][]byte{
			"arguments.json":    []byte(`{"nodeMonitorGracePeriod":40}`),
			"featureGates.json": []byte(`{"kubelet":["SomeGate"]}`),
		},
	})

	service := &bashiblecontext.Service{
		Client: fake.NewClientBuilder().
			WithScheme(tenantScheme(t)).
			WithRuntimeObjects(objects...).
			Build(),
		RootCAFile: filepath.Join(t.TempDir(), "no-such-ca.crt"),
	}

	inputs, _ := usableBashibleExternalInputs(string(readCPMGolden(t, "external-inputs.golden.yaml")))
	require.NotNil(t, inputs)

	assembled, err := service.Build(context.Background(), service.ReadGlobals(context.Background()), nil)
	require.NoError(t, err)
	applyBashibleExternalInputs(assembled, inputs)

	raw, err := bashiblecontext.Marshal(assembled)
	require.NoError(t, err)

	got := asDocument(t, raw)
	parent := asDocument(t, readCPMGolden(t, "context.golden.yaml"))

	var extra []string
	for key := range got {
		if _, ok := parent[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	require.Equal(t, []string{"allowedKubeletFeatureGates", "nodeStatusUpdateFrequency"}, extra)
}

func TestUsableBashibleExternalInputs(t *testing.T) {
	published := string(readCPMGolden(t, "external-inputs.golden.yaml"))

	t.Run("published", func(t *testing.T) {
		inputs, reason := usableBashibleExternalInputs(published)
		require.NotNil(t, inputs)
		require.Empty(t, reason)
	})

	t.Run("nothing but a version", func(t *testing.T) {
		inputs, reason := usableBashibleExternalInputs("version: 1\n")
		require.Nil(t, inputs)
		require.Contains(t, reason, "no kubernetes CA")
	})

	t.Run("unknown version", func(t *testing.T) {
		inputs, reason := usableBashibleExternalInputs("version: 99\n")
		require.Nil(t, inputs)
		require.Contains(t, reason, "not supported")
	})

	t.Run("unreadable", func(t *testing.T) {
		inputs, reason := usableBashibleExternalInputs("\tnot yaml")
		require.Nil(t, inputs)
		require.Contains(t, reason, "unreadable")
	})
}

// TestKeepPublishedContext covers the failure path of every early return in the handler: the
// Secret is owned by the chart now, so an unset value deletes it and leaves bashible-apiserver
// with no context to serve.
func TestKeepPublishedContext(t *testing.T) {
	published := "clusterDomain: cluster.virtual\n"

	newInput := func(snapshots []sdkpkg.Snapshot) (*go_hook.HookInput, *sdkpatchablevalues.PatchableValues) {
		values, err := sdkpatchablevalues.NewPatchableValues(map[string]interface{}{
			"nodeManager": map[string]interface{}{"internal": map[string]interface{}{}},
		})
		require.NoError(t, err)

		return &go_hook.HookInput{
			Values:    values,
			Snapshots: stubSnapshots{"published": snapshots},
			Logger:    log.NewNop(),
		}, values
	}

	t.Run("a published context is put back", func(t *testing.T) {
		input, values := newInput([]sdkpkg.Snapshot{stubSnapshot{value: published}})
		require.NoError(t, keepPublishedContext(input, "some reason"))

		patches := values.GetPatches()
		require.Len(t, patches, 1)
		require.Equal(t, "/nodeManager/internal/bashibleContext", patches[0].Path)

		raw, err := json.Marshal(patches[0].Value)
		require.NoError(t, err)
		var got string
		require.NoError(t, json.Unmarshal(raw, &got))
		require.Equal(t, published, got)
	})

	// A tenant that has never had a context: there is no Secret to lose, and publishing an
	// empty string would render one holding nothing.
	t.Run("nothing published yet", func(t *testing.T) {
		for name, snapshots := range map[string][]sdkpkg.Snapshot{
			"no secret":    nil,
			"empty secret": {stubSnapshot{value: ""}},
		} {
			t.Run(name, func(t *testing.T) {
				input, values := newInput(snapshots)
				require.NoError(t, keepPublishedContext(input, "some reason"))
				require.Empty(t, values.GetPatches())
			})
		}
	})
}

func equalYAML(t *testing.T, a, b interface{}) bool {
	t.Helper()

	rawA, err := yaml.Marshal(a)
	require.NoError(t, err)
	rawB, err := yaml.Marshal(b)
	require.NoError(t, err)
	return string(rawA) == string(rawB)
}
