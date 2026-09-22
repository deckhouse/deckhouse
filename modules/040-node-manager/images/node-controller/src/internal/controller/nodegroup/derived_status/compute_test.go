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

package derived_status

import (
	"context"
	"hash/fnv"
	"strconv"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func boolPtr(b bool) *bool { return &b }

func mustSemver(t *testing.T, s string) *semver.Version {
	t.Helper()
	v, err := semver.NewVersion(s)
	require.NoError(t, err)
	return v
}

func TestDefaultCloudEphemeralEngine(t *testing.T) {
	cases := []struct {
		name   string
		reg    CloudProviderRegistration
		useMCM bool
		want   string
	}{
		{
			name: "neither MCM nor CAPI",
			reg:  CloudProviderRegistration{},
			want: engineNone,
		},
		{
			name: "MCM only",
			reg:  CloudProviderRegistration{MachineClassKind: "AWSInstanceClass"},
			want: engineMCM,
		},
		{
			name: "CAPI only",
			reg:  CloudProviderRegistration{CAPIClusterKind: "DVPCluster"},
			want: engineCAPI,
		},
		{
			name:   "both, useMCM=false defaults to CAPI",
			reg:    CloudProviderRegistration{MachineClassKind: "AWSInstanceClass", CAPIClusterKind: "DVPCluster"},
			useMCM: false,
			want:   engineCAPI,
		},
		{
			name:   "both, useMCM=true forces MCM",
			reg:    CloudProviderRegistration{MachineClassKind: "AWSInstanceClass", CAPIClusterKind: "DVPCluster"},
			useMCM: true,
			want:   engineMCM,
		},
		{
			name: "empty-string kinds are treated as absent",
			reg:  CloudProviderRegistration{},
			want: engineNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, defaultCloudEphemeralEngine(tc.reg, tc.useMCM))
		})
	}
}

func TestEngine_PinAndDefaults(t *testing.T) {
	mcmProvider := CloudProviderRegistration{MachineClassKind: "AWSInstanceClass"}
	capiProvider := CloudProviderRegistration{CAPIClusterKind: "DVPCluster"}

	cases := []struct {
		name string
		ng   *v1.NodeGroup
		reg  CloudProviderRegistration
		want string
	}{
		{
			name: "status.engine short-circuits regardless of provider",
			ng: &v1.NodeGroup{
				Status: v1.NodeGroupStatus{Engine: engineMCM},
				Spec:   v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
			},
			reg:  capiProvider,
			want: engineMCM,
		},
		{
			name: "cloud ephemeral resolves from provider (MCM)",
			ng:   &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}},
			reg:  mcmProvider,
			want: engineMCM,
		},
		{
			name: "cloud ephemeral with use-mcm annotation forces MCM over CAPI",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{useMCMAnnotation: "true"}},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
			},
			reg:  CloudProviderRegistration{MachineClassKind: "AWSInstanceClass", CAPIClusterKind: "DVPCluster"},
			want: engineMCM,
		},
		{
			name: "static with staticInstances is CAPI",
			ng: &v1.NodeGroup{Spec: v1.NodeGroupSpec{
				NodeType:        v1.NodeTypeStatic,
				StaticInstances: &v1.StaticInstancesSpec{},
			}},
			want: engineCAPI,
		},
		{
			name: "static without staticInstances is None",
			ng:   &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic}},
			want: engineNone,
		},
		{
			name: "cloud static is None",
			ng:   &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudStatic}},
			want: engineNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, engineFrom(tc.ng, tc.reg, machineDeployments{}))
		})
	}
}

func TestCalculateUpdateEpoch(t *testing.T) {
	const uuid = "cluster-uuid-1"
	const ng = "worker"

	drift := func(clusterUUID, nodeGroup string) int64 {
		h := fnv.New64a()
		_, _ = h.Write([]byte(clusterUUID))
		_, _ = h.Write([]byte(nodeGroup))
		return int64(h.Sum64() % uint64(epochWindowSize))
	}

	t.Run("deterministic for same inputs", func(t *testing.T) {
		a := calculateUpdateEpoch(1_700_000_000, uuid, ng)
		b := calculateUpdateEpoch(1_700_000_000, uuid, ng)
		assert.Equal(t, a, b)
	})

	t.Run("ts<=drift returns the drift itself", func(t *testing.T) {
		d := drift(uuid, ng)
		got := calculateUpdateEpoch(0, uuid, ng)
		assert.Equal(t, strconv.FormatInt(d, 10), got)
	})

	t.Run("epoch is the next window boundary shifted by drift", func(t *testing.T) {
		d := drift(uuid, ng)
		ts := int64(1_700_000_000)
		got, err := strconv.ParseInt(calculateUpdateEpoch(ts, uuid, ng), 10, 64)
		require.NoError(t, err)
		assert.Greater(t, got, ts, "epoch must be in the future for ts>drift")
		assert.LessOrEqual(t, got, ts+epochWindowSize, "epoch is at most one window ahead")
		assert.Zero(t, (got-d)%epochWindowSize, "epoch minus drift lands on a window boundary")
	})

	t.Run("per-nodegroup drift differs", func(t *testing.T) {
		assert.NotEqual(t, drift(uuid, "worker"), drift(uuid, "master-set-that-hashes-apart"))
	})
}

func TestResolveCRIType(t *testing.T) {
	v118 := mustSemver(t, "1.18.0")
	v129 := mustSemver(t, "1.29.0")

	cases := []struct {
		name         string
		ng           *v1.NodeGroup
		effectiveVer *semver.Version
		defaultCRI   string
		want         string
		wantErr      bool
	}{
		{
			name:         "default containerd on modern k8s",
			ng:           &v1.NodeGroup{},
			effectiveVer: v129,
			want:         criTypeContainerd,
		},
		{
			name:         "default docker on pre-1.19",
			ng:           &v1.NodeGroup{},
			effectiveVer: v118,
			want:         criTypeDocker,
		},
		{
			name:         "explicit containerd on pre-1.19 errors",
			ng:           &v1.NodeGroup{Spec: v1.NodeGroupSpec{CRI: &v1.CRISpec{Type: v1.CRITypeContainerd}}},
			effectiveVer: v118,
			wantErr:      true,
		},
		{
			name:         "docker with manage=false becomes NotManaged",
			ng:           &v1.NodeGroup{Spec: v1.NodeGroupSpec{CRI: &v1.CRISpec{Type: v1.CRITypeDocker, Docker: &v1.DockerSpec{Manage: boolPtr(false)}}}},
			effectiveVer: v129,
			want:         criTypeNotManaged,
		},
		{
			name:         "docker with manage=true stays Docker",
			ng:           &v1.NodeGroup{Spec: v1.NodeGroupSpec{CRI: &v1.CRISpec{Type: v1.CRITypeDocker, Docker: &v1.DockerSpec{Manage: boolPtr(true)}}}},
			effectiveVer: v129,
			want:         criTypeDocker,
		},
		{
			name:         "defaultCRI from cluster config wins over version default",
			ng:           &v1.NodeGroup{},
			effectiveVer: v129,
			defaultCRI:   criTypeNotManaged,
			want:         criTypeNotManaged,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveCRIType(tc.ng, tc.effectiveVer, tc.defaultCRI)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSerializeTaints(t *testing.T) {
	t.Run("nil node template", func(t *testing.T) {
		assert.Empty(t, serializeTaints(&v1.NodeGroup{}))
	})
	t.Run("empty taints", func(t *testing.T) {
		ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{}}}
		assert.Empty(t, serializeTaints(ng))
	})
	t.Run("multiple taints joined by comma in order", func(t *testing.T) {
		ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeTemplate: &v1.NodeTemplate{Taints: []corev1.Taint{
			{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
			{Key: "reserved", Effect: corev1.TaintEffectNoExecute},
		}}}}
		assert.Equal(t, "dedicated=gpu:NoSchedule,reserved:NoExecute", serializeTaints(ng))
	})
}

func TestEngine_ExistingMachineDeploymentsWin(t *testing.T) {
	bothCapable := CloudProviderRegistration{
		MachineClassKind: "YandexMachineClass",
		CAPIClusterKind:  "YandexCluster",
	}

	cloudEphemeral := func() *v1.NodeGroup {
		return &v1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
		}
	}

	t.Run("MCM machines keep an unpinned group on MCM, against the CAPI default", func(t *testing.T) {
		assert.Equal(t, engineMCM, engineFrom(cloudEphemeral(), bothCapable, machineDeployments{MCM: true}))
	})

	t.Run("CAPI machines answer CAPI", func(t *testing.T) {
		assert.Equal(t, engineCAPI, engineFrom(cloudEphemeral(), bothCapable, machineDeployments{CAPI: true}))
	})

	// The only case where the CAPI branch changes the answer: the annotation would say MCM, but
	// the group demonstrably runs CAPI machines, and an annotation does not migrate a live group.
	t.Run("CAPI machines outrank the use-mcm annotation", func(t *testing.T) {
		ng := cloudEphemeral()
		ng.Annotations = map[string]string{useMCMAnnotation: "true"}
		assert.Equal(t, engineCAPI, engineFrom(ng, bothCapable, machineDeployments{CAPI: true}))
	})

	t.Run("MCM wins when both kinds are present", func(t *testing.T) {
		live := machineDeployments{MCM: true, CAPI: true}
		assert.Equal(t, engineMCM, engineFrom(cloudEphemeral(), bothCapable, live))
	})

	t.Run("no machines at all falls back to the provider default", func(t *testing.T) {
		assert.Equal(t, engineCAPI, engineFrom(cloudEphemeral(), bothCapable, machineDeployments{}))
	})

	// The pin is what the group was already told to be; machines that contradict it are the
	// symptom of something else and must not flip the engine, which would recreate every node.
	t.Run("the pin outranks the machines", func(t *testing.T) {
		ng := cloudEphemeral()
		ng.Status.Engine = engineCAPI
		assert.Equal(t, engineCAPI, engineFrom(ng, bothCapable, machineDeployments{MCM: true}))
	})

	t.Run("use-mcm decides a group that has no machines yet", func(t *testing.T) {
		ng := cloudEphemeral()
		ng.Annotations = map[string]string{useMCMAnnotation: "true"}
		assert.Equal(t, engineMCM, engineFrom(ng, bothCapable, machineDeployments{}))
	})

	t.Run("static groups never consult machines", func(t *testing.T) {
		ng := cloudEphemeral()
		ng.Spec.NodeType = v1.NodeTypeStatic
		assert.Equal(t, engineNone, engineFrom(ng, bothCapable, machineDeployments{MCM: true}))
	})
}

func TestEngineUndecided(t *testing.T) {
	bothCapable := CloudProviderRegistration{MachineClassKind: "YandexMachineClass", CAPIClusterKind: "YandexCluster"}
	capiOnly := CloudProviderRegistration{CAPIClusterKind: "DVPCluster"}

	ng := &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}}
	assert.True(t, engineUndecided(ng, bothCapable), "an unpinned cloud group on a both-kinds provider is the only case worth a list")
	assert.False(t, engineUndecided(ng, capiOnly), "a single-kind provider has no default to second-guess")

	pinned := ng.DeepCopy()
	pinned.Status.Engine = engineMCM
	assert.False(t, engineUndecided(pinned, bothCapable))

	static := ng.DeepCopy()
	static.Spec.NodeType = v1.NodeTypeStatic
	assert.False(t, engineUndecided(static, bothCapable))
}

// An apiserver that does not serve a MachineDeployment kind must not read as "no machines": that
// would pin the provider default on a group that may run on the other engine, irreversibly.
func TestResolveEngine_UnservedKindIsAnError(t *testing.T) {
	bothCapable := CloudProviderRegistration{MachineClassKind: "YandexMachineClass", CAPIClusterKind: "YandexCluster"}
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
	}

	_, err := ResolveEngine(t.Context(), noMatchReader{}, ng, bothCapable)
	require.Error(t, err)
	require.True(t, meta.IsNoMatchError(err), "the no-match must survive wrapping: %v", err)
}

// noMatchReader answers every List the way a RESTMapper does for a kind the apiserver does not
// serve. Get is never reached by ResolveEngine.
type noMatchReader struct{ client.Reader }

func (noMatchReader) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	gvk := list.GetObjectKind().GroupVersionKind()
	return &meta.NoKindMatchError{GroupKind: gvk.GroupKind(), SearchedVersions: []string{gvk.Version}}
}
