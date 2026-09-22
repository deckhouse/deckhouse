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

package licensing

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func nodeName(i int) string { return fmt.Sprintf("worker-%03d", i) }

func updateOf(before, after client.Object) event.UpdateEvent {
	return event.UpdateEvent{ObjectOld: before, ObjectNew: after}
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func effectiveStatusOf(t *testing.T, env *testEnv) v1alpha1.EffectiveLicenseStatus {
	t.Helper()

	var effective v1alpha1.EffectiveLicense
	key := types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}
	if err := env.cl.Get(context.Background(), key, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	return effective.Status
}

// serverNodes is the published allocation, server group only.
func serverNodes(t *testing.T, env *testEnv) []string {
	t.Helper()

	var out []string
	for _, node := range effectiveStatusOf(t, env).Nodes {
		if node.Billing == v1alpha1.LicenseNodeServer {
			out = append(out, node.Name)
		}
	}
	return out
}

func assertNodes(t *testing.T, nodes []v1alpha1.LicenseNodeStatus, want map[string]v1alpha1.LicenseNodeBilling) {
	t.Helper()

	got := make(map[string]v1alpha1.LicenseNodeBilling, len(nodes))
	for _, node := range nodes {
		got[node.Name] = node.Billing
		if node.Billing == v1alpha1.LicenseNodeFree && node.Reason == "" {
			t.Fatalf("free node %s carries no reason", node.Name)
		}
		if node.Billing != v1alpha1.LicenseNodeFree && node.Reason != "" {
			t.Fatalf("node %s is billed as %s but carries a reason %q", node.Name, node.Billing, node.Reason)
		}
	}
	for name, billing := range want {
		if got[name] != billing {
			t.Fatalf("node %s is %q, want %q (all: %v)", name, got[name], billing, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("status lists %d nodes, want %d", len(got), len(want))
	}
}
