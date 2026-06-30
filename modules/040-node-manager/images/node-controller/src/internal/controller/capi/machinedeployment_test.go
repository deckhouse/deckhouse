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

package capi

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func ptr[T any](v T) *T { return &v }

func TestCalculateReplicas(t *testing.T) {
	cases := []struct {
		name                    string
		current, min, max, want int32
	}{
		{"min>=max collapses to max", 5, 3, 3, 3},
		{"min>max collapses to max", 5, 4, 2, 2},
		{"current zero bumps to min", 0, 2, 5, 2},
		{"current below min bumps to min", 1, 2, 5, 2},
		{"current equals min stays min", 2, 2, 5, 2},
		{"current above max clamps to max", 9, 2, 5, 5},
		{"current within range preserved", 3, 2, 5, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := calculateReplicas(c.current, c.min, c.max); got != c.want {
				t.Fatalf("calculateReplicas(%d,%d,%d)=%d, want %d", c.current, c.min, c.max, got, c.want)
			}
		})
	}
}

func TestGetMinMax(t *testing.T) {
	t.Run("cloud instances", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{MinPerZone: 2, MaxPerZone: 7}
		min, max := getMinMax(ng)
		if min != 2 || max != 7 {
			t.Fatalf("got min=%d max=%d, want 2/7", min, max)
		}
	})

	t.Run("static instances pin min==max==count", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Spec.StaticInstances = &deckhousev1.StaticInstancesSpec{Count: ptr(int32(4))}
		min, max := getMinMax(ng)
		if min != 4 || max != 4 {
			t.Fatalf("got min=%d max=%d, want 4/4", min, max)
		}
	})

	t.Run("static takes precedence over cloud", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{MinPerZone: 1, MaxPerZone: 9}
		ng.Spec.StaticInstances = &deckhousev1.StaticInstancesSpec{Count: ptr(int32(3))}
		min, max := getMinMax(ng)
		if min != 3 || max != 3 {
			t.Fatalf("got min=%d max=%d, want 3/3", min, max)
		}
	})

	t.Run("nothing set yields zeros", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		min, max := getMinMax(ng)
		if min != 0 || max != 0 {
			t.Fatalf("got min=%d max=%d, want 0/0", min, max)
		}
	})
}

func TestIntOrDefault(t *testing.T) {
	if got := intOrDefault(nil, 1); got != 1 {
		t.Fatalf("nil pointer: got %d, want default 1", got)
	}
	if got := intOrDefault(ptr(int32(0)), 1); got != 0 {
		t.Fatalf("explicit zero must override default: got %d, want 0", got)
	}
	if got := intOrDefault(ptr(int32(5)), 1); got != 5 {
		t.Fatalf("got %d, want 5", got)
	}
}

func TestSha256Hash(t *testing.T) {
	const uuid = "11111111-2222-3333-4444-555555555555"
	const zone = "ru-central1-a"
	const checksum = "b917b120e438be362a69dfbfd9b8efd977820338a71b930fde597ba909feee07"

	if got := sha256Hash(uuid + zone); len(got) != 8 {
		t.Fatalf("hash length=%d, want 8", len(got))
	}
	if a, b := sha256Hash(uuid+zone), sha256Hash(uuid+zone); a != b {
		t.Fatalf("hash not deterministic: %s != %s", a, b)
	}

	mdHash := sha256Hash(uuid + zone)
	templateHash := sha256Hash(uuid + zone + checksum)
	if mdHash == templateHash {
		t.Fatalf("md and template hashes must differ, both=%s", mdHash)
	}

	if sha256Hash(uuid+"zone-a") == sha256Hash(uuid+"zone-b") {
		t.Fatal("different zones must produce different hashes")
	}
}

func TestSerializeNodeGroupLabels(t *testing.T) {
	t.Run("injects the three standard labels", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Name = "worker"
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		got := serializeNodeGroupLabels(ng)
		for _, want := range []string{
			"node.deckhouse.io/group=worker",
			"node.deckhouse.io/type=CloudEphemeral",
			"node-role.kubernetes.io/worker=",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("labels %q missing %q", got, want)
			}
		}
	})

	t.Run("merges custom NodeTemplate labels", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Name = "worker"
		ng.Spec.NodeType = deckhousev1.NodeTypeStatic
		ng.Spec.NodeTemplate = &deckhousev1.NodeTemplate{
			Labels: map[string]string{"custom": "yes"},
		}
		got := serializeNodeGroupLabels(ng)
		if !strings.Contains(got, "custom=yes") {
			t.Fatalf("labels %q missing custom=yes", got)
		}
	})
}

func TestSerializeNodeGroupTaints(t *testing.T) {
	t.Run("nil NodeTemplate yields empty", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		if got := serializeNodeGroupTaints(ng); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("empty taints yields empty", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Spec.NodeTemplate = &deckhousev1.NodeTemplate{}
		if got := serializeNodeGroupTaints(ng); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("taints are serialized and sorted", func(t *testing.T) {
		ng := &deckhousev1.NodeGroup{}
		ng.Spec.NodeTemplate = &deckhousev1.NodeTemplate{
			Taints: []corev1.Taint{
				{Key: "b", Value: "2", Effect: corev1.TaintEffectNoSchedule},
				{Key: "a", Value: "1", Effect: corev1.TaintEffectNoExecute},
			},
		}
		got := serializeNodeGroupTaints(ng)
		parts := strings.Split(got, ",")
		if len(parts) != 2 {
			t.Fatalf("got %q, want 2 taints", got)
		}
		if parts[0] >= parts[1] {
			t.Fatalf("taints not sorted: %q", got)
		}
	})
}
