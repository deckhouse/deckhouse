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
	"hash/fnv"
	"strconv"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		name          string
		cloudProvider map[string]interface{}
		useMCM        bool
		want          string
	}{
		{
			name:          "neither MCM nor CAPI",
			cloudProvider: map[string]interface{}{},
			want:          engineNone,
		},
		{
			name:          "MCM only",
			cloudProvider: map[string]interface{}{"machineClassKind": "AWSInstanceClass"},
			want:          engineMCM,
		},
		{
			name:          "CAPI only",
			cloudProvider: map[string]interface{}{"capiClusterKind": "DVPCluster"},
			want:          engineCAPI,
		},
		{
			name:          "both, useMCM=false defaults to CAPI",
			cloudProvider: map[string]interface{}{"machineClassKind": "AWSInstanceClass", "capiClusterKind": "DVPCluster"},
			useMCM:        false,
			want:          engineCAPI,
		},
		{
			name:          "both, useMCM=true forces MCM",
			cloudProvider: map[string]interface{}{"machineClassKind": "AWSInstanceClass", "capiClusterKind": "DVPCluster"},
			useMCM:        true,
			want:          engineMCM,
		},
		{
			name:          "empty-string kinds are treated as absent",
			cloudProvider: map[string]interface{}{"machineClassKind": "", "capiClusterKind": ""},
			want:          engineNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, defaultCloudEphemeralEngine(tc.cloudProvider, tc.useMCM))
		})
	}
}

func TestComputeEngine(t *testing.T) {
	mcmProvider := map[string]interface{}{"machineClassKind": "AWSInstanceClass"}
	capiProvider := map[string]interface{}{"capiClusterKind": "DVPCluster"}

	cases := []struct {
		name          string
		ng            *v1.NodeGroup
		cloudProvider map[string]interface{}
		want          string
	}{
		{
			name: "status.engine short-circuits regardless of provider",
			ng: &v1.NodeGroup{
				Status: v1.NodeGroupStatus{Engine: engineMCM},
				Spec:   v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
			},
			cloudProvider: capiProvider,
			want:          engineMCM,
		},
		{
			name:          "cloud ephemeral resolves from provider (MCM)",
			ng:            &v1.NodeGroup{Spec: v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral}},
			cloudProvider: mcmProvider,
			want:          engineMCM,
		},
		{
			name: "cloud ephemeral with use-mcm annotation forces MCM over CAPI",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{useMCMAnnotation: "true"}},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral},
			},
			cloudProvider: map[string]interface{}{"machineClassKind": "AWSInstanceClass", "capiClusterKind": "DVPCluster"},
			want:          engineMCM,
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
			s := &Service{}
			assert.Equal(t, tc.want, s.computeEngine(tc.ng, tc.cloudProvider))
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
