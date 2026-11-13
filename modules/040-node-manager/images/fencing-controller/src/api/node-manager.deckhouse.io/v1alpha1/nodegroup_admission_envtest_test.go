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

package v1alpha1_test

import (
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"fencing-controller/internal/testenv"
)

const criticalModeDenial = "The Critical fencing mode is available only in the commercial editions of Deckhouse"

// TestCriticalModeAdmissionEnvtest sends the writes of NodeGroups to a real API
// server with the NodeGroup CRD and the admission policy of CE. The groups exist
// before the policy, as in a cluster after switching the edition to CE. The writes are dry runs,
// so each case starts from the same objects.
func TestCriticalModeAdmissionEnvtest(t *testing.T) {
	env := testenv.Start(t, "crds/node_group.yaml")
	api := env.Client
	ctx := t.Context()

	for name, mode := range map[string]string{"critical": "Critical", "normal": "Normal", "no-fencing": ""} {
		if err := api.Create(ctx, nodeGroup(name, mode)); err != nil {
			t.Fatalf("create nodegroup %s: %v", name, err)
		}
	}

	for _, obj := range testenv.Manifests(t, "templates/fencing-agent/critical-mode-policy.yaml") {
		if err := api.Create(ctx, obj); err != nil {
			t.Fatalf("create %s %s: %v", obj.GetKind(), obj.GetName(), err)
		}
	}

	// The policy starts to work some time after its creation.
	for deadline := time.Now().Add(30 * time.Second); api.Create(ctx, nodeGroup("new", "Critical"), client.DryRunAll) == nil; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("admission policy does not deny a new NodeGroup with the Critical mode")
		}
	}

	for _, tc := range []struct {
		name string
		// update is the group that the case changes. Empty means a new group.
		update string
		mode   string
		denied bool
	}{
		{name: "create with Critical", mode: "Critical", denied: true},
		{name: "create with Normal", mode: "Normal"},
		{name: "create with Slow", mode: "Slow"},
		{name: "create without fencing"},
		{name: "change of another field of a Critical group", update: "critical", mode: "Critical"},
		{name: "change from Critical to Normal", update: "critical", mode: "Normal"},
		{name: "change from Normal to Critical", update: "normal", mode: "Critical", denied: true},
		{name: "change from Normal to Slow", update: "normal", mode: "Slow"},
		{name: "change of a group without fencing to Critical", update: "no-fencing", mode: "Critical", denied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.update == "" {
				err = api.Create(ctx, nodeGroup("new", tc.mode), client.DryRunAll)
			} else {
				ng := &unstructured.Unstructured{}
				ng.SetAPIVersion("deckhouse.io/v1")
				ng.SetKind("NodeGroup")
				if err := api.Get(ctx, types.NamespacedName{Name: tc.update}, ng); err != nil {
					t.Fatalf("get nodegroup %s: %v", tc.update, err)
				}
				ng.SetLabels(map[string]string{"changed": "true"})
				if err := unstructured.SetNestedField(ng.Object, tc.mode, "spec", "fencing", "mode"); err != nil {
					t.Fatalf("set mode: %v", err)
				}
				err = api.Update(ctx, ng, client.DryRunAll)
			}

			switch {
			case !tc.denied && err != nil:
				t.Errorf("write returned %v, want it accepted", err)
			case tc.denied && (!apierrors.IsForbidden(err) || !strings.Contains(err.Error(), criticalModeDenial)):
				t.Errorf("write returned %v, want the denial of the admission policy", err)
			}
		})
	}
}

// nodeGroup returns a Static NodeGroup. An empty mode gives a group without
// spec.fencing.
func nodeGroup(name, mode string) *unstructured.Unstructured {
	spec := map[string]any{"nodeType": "Static"}
	if mode != "" {
		spec["fencing"] = map[string]any{"mode": mode}
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}
