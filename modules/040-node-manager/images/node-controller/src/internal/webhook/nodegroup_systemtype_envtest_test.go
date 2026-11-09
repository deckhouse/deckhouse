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

package webhook

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	systemTypeChangedMessage = ".spec.systemType DeckhouseEngine cannot be changed or removed"
	cloudStaticMessage       = "systemType DeckhouseEngine is not supported for nodeType CloudStatic"
	staticInstancesMessage   = "staticInstances cannot provision a DeckhouseEngine node. Add machines with d8 node add"
)

// The CRD rules on systemType, run by a real apiserver: no webhook is installed
// here, so whatever is refused below is refused by the CRD alone.
func TestNodeGroupSystemTypeCRDRules(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` or set KUBEBUILDER_ASSETS")
	}
	env, _, c, err := testenv.Start(runtime.NewScheme(), testenv.CRDPaths(testenv.WithNodeGroupCRDFile())...)
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})

	t.Run("create", func(t *testing.T) {
		tests := []struct {
			version string
			name    string
			spec    map[string]any
			wantErr string
		}{
			{version: "v1", name: "Immutable", spec: map[string]any{"nodeType": "Static", "systemType": "Immutable"}, wantErr: legacyRefusal("Immutable")},
			{version: "v1", name: "Mutable", spec: map[string]any{"nodeType": "CloudStatic", "systemType": "Mutable"}, wantErr: legacyRefusal("Mutable")},
			{version: "v1alpha2", name: "Immutable", spec: map[string]any{"nodeType": "Static", "systemType": "Immutable"}, wantErr: legacyRefusal("Immutable")},
			{version: "v1alpha1", name: "Mutable", spec: map[string]any{"nodeType": "Static", "systemType": "Mutable"}, wantErr: legacyRefusal("Mutable")},
			{
				version: "v1",
				name:    "DeckhouseEngine on CloudStatic",
				spec:    map[string]any{"nodeType": "CloudStatic", "systemType": "DeckhouseEngine"},
				wantErr: cloudStaticMessage,
			},
			{
				version: "v1",
				name:    "DeckhouseEngine with staticInstances",
				spec:    map[string]any{"nodeType": "Static", "systemType": "DeckhouseEngine", "staticInstances": map[string]any{"count": int64(1)}},
				wantErr: staticInstancesMessage,
			},
			{version: "v1", name: "GenericOS on CloudStatic", spec: map[string]any{"nodeType": "CloudStatic", "systemType": "GenericOS"}},
			{version: "v1", name: "staticInstances without systemType", spec: map[string]any{"nodeType": "Static", "staticInstances": map[string]any{"count": int64(1)}}},
		}
		for _, tc := range tests {
			t.Run(tc.version+": "+tc.name, func(t *testing.T) {
				ng := nodeGroupObject(tc.version, testenv.UniqueName("ng"), tc.spec)
				err := c.Create(t.Context(), ng)
				if err == nil {
					t.Cleanup(func() { _ = c.Delete(t.Context(), ng) })
				}
				checkRefusal(t, err, tc.wantErr)
			})
		}
	})

	// Empty and GenericOS mean the same thing, so moving between them is free; a
	// group may still become DeckhouseEngine (the webhook guards its bashible nodes);
	// only DeckhouseEngine is final.
	t.Run("update", func(t *testing.T) {
		tests := []struct {
			version string
			from    string
			to      string
			wantErr string
		}{
			{version: "v1", from: "", to: "GenericOS"},
			{version: "v1", from: "GenericOS", to: ""},
			{version: "v1", from: "GenericOS", to: "DeckhouseEngine"},
			{version: "v1", from: "DeckhouseEngine", to: "GenericOS", wantErr: systemTypeChangedMessage},
			{version: "v1", from: "DeckhouseEngine", to: "", wantErr: systemTypeChangedMessage},
			{version: "v1", from: "", to: "Mutable", wantErr: legacyRefusal("Mutable")},
			{version: "v1", from: "GenericOS", to: "Immutable", wantErr: legacyRefusal("Immutable")},
			{version: "v1", from: "DeckhouseEngine", to: "Immutable", wantErr: legacyRefusal("Immutable")},
			{version: "v1", from: "Mutable", to: "GenericOS"},
			{version: "v1", from: "Mutable", to: ""},
			{version: "v1", from: "Mutable", to: "DeckhouseEngine"},
			{version: "v1", from: "Mutable", to: "Immutable", wantErr: legacyRefusal("Immutable")},
			{version: "v1", from: "Immutable", to: "DeckhouseEngine"},
			{version: "v1", from: "Immutable", to: "GenericOS", wantErr: systemTypeChangedMessage},
			{version: "v1", from: "Immutable", to: "", wantErr: systemTypeChangedMessage},
			{version: "v1", from: "Immutable", to: "Mutable", wantErr: legacyRefusal("Mutable")},
			{version: "v1alpha2", from: "Immutable", to: "", wantErr: systemTypeChangedMessage},
			{version: "v1alpha2", from: "GenericOS", to: "Immutable", wantErr: legacyRefusal("Immutable")},
			{version: "v1alpha1", from: "Immutable", to: "DeckhouseEngine"},
			{version: "v1alpha1", from: "DeckhouseEngine", to: "GenericOS", wantErr: systemTypeChangedMessage},
		}
		for _, tc := range tests {
			t.Run(tc.version+": "+systemTypeName(tc.from)+" to "+systemTypeName(tc.to), func(t *testing.T) {
				ng := storeNodeGroup(t, c, nodeGroupObject(tc.version, testenv.UniqueName("ng"), withSystemType(map[string]any{"nodeType": "Static"}, tc.from)))

				spec, _, err := unstructured.NestedMap(ng.Object, "spec")
				if err != nil {
					t.Fatalf("read spec: %v", err)
				}
				if err := unstructured.SetNestedMap(ng.Object, withSystemType(spec, tc.to), "spec"); err != nil {
					t.Fatalf("write spec: %v", err)
				}
				checkRefusal(t, c.Update(t.Context(), ng), tc.wantErr)
			})
		}
	})

	t.Run("a group stored in a legacy spelling", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			change  func(spec map[string]any)
			wantErr string
		}{
			{name: "takes a change to another field", change: func(spec map[string]any) { spec["nodeDrainTimeoutSecond"] = int64(600) }},
			{name: "is still a DeckhouseEngine group to the rules", change: func(spec map[string]any) {
				spec["staticInstances"] = map[string]any{"count": int64(1)}
			}, wantErr: staticInstancesMessage},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ng := storeNodeGroup(t, c, nodeGroupObject("v1", testenv.UniqueName("ng"), map[string]any{"nodeType": "Static", "systemType": "Immutable"}))

				spec, _, err := unstructured.NestedMap(ng.Object, "spec")
				if err != nil {
					t.Fatalf("read spec: %v", err)
				}
				tc.change(spec)
				if err := unstructured.SetNestedMap(ng.Object, spec, "spec"); err != nil {
					t.Fatalf("write spec: %v", err)
				}
				checkRefusal(t, c.Update(t.Context(), ng), tc.wantErr)
			})
		}
	})
}

func storeNodeGroup(t *testing.T, c client.Client, ng *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	var err error
	if systemType, _, _ := unstructured.NestedString(ng.Object, "spec", "systemType"); systemType == "Mutable" || systemType == "Immutable" {
		err = testenv.CreateLegacyNodeGroup(t.Context(), c, ng)
	} else {
		err = c.Create(t.Context(), ng)
	}
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = c.Delete(t.Context(), ng) })
	return ng
}

func legacyRefusal(systemType string) string {
	return `Unsupported value: "` + systemType + `"`
}

func nodeGroupObject(version, name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/" + version,
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}

func withSystemType(spec map[string]any, systemType string) map[string]any {
	delete(spec, "systemType")
	if systemType != "" {
		spec["systemType"] = systemType
	}
	return spec
}

func checkRefusal(t *testing.T, err error, wantErr string) {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("want accepted, got: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("want refused with %q, got: %v", wantErr, err)
	}
}

func systemTypeName(systemType string) string {
	if systemType == "" {
		return "unset"
	}
	return systemType
}
