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
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func fencingNodeGroupFromYAML(t *testing.T, manifest string) fencingNodeGroup {
	t.Helper()

	obj := map[string]any{}
	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	res, err := fencingFilterNG(&unstructured.Unstructured{Object: obj})
	if err != nil {
		t.Fatalf("filter node group: %v", err)
	}

	return res.(fencingNodeGroup)
}

// The narrow hook only reruns when its filter result changes, so fields outside
// spec.fencing must not leak into it.
func TestFencingFilterIgnoresUnrelatedSpecFields(t *testing.T) {
	containerd := fencingNodeGroupFromYAML(t, `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: Containerd
  nodeTemplate:
    labels:
      ship-class: frigate
  fencing:
    mode: Watchdog
`)

	notManaged := fencingNodeGroupFromYAML(t, `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: NotManaged
  nodeTemplate:
    labels:
      ship-class: destroyer
      extra: label
  fencing:
    mode: Watchdog
`)

	if containerd != notManaged {
		t.Fatalf("filter result changed with unrelated spec fields: %+v != %+v", containerd, notManaged)
	}

	want := fencingNodeGroup{Name: "worker", Mode: "Watchdog", WatchdogTimeout: 60}
	if containerd != want {
		t.Fatalf("unexpected filter result: %+v, want %+v", containerd, want)
	}
}

func TestFencingFilterSkipsNodeGroupWithoutFencing(t *testing.T) {
	obj := map[string]any{}
	if err := yaml.Unmarshal([]byte(`
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
`), &obj); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	res, err := fencingFilterNG(&unstructured.Unstructured{Object: obj})
	if err != nil {
		t.Fatalf("filter node group: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil filter result, got %+v", res)
	}
}

var _ = Describe("Modules :: node-manager :: hooks :: fencing_node_groups ::", func() {
	const nodeGroups = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-watchdog
spec:
  nodeType: Static
  fencing:
    mode: Watchdog
    watchdog:
      timeout: 45s
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-notify
spec:
  nodeType: Static
  fencing:
    mode: Notify
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-plain
spec:
  nodeType: Static
`

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Two NodeGroups with fencing and one without", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeGroups))
			f.RunHook()
		})

		It("writes only the fencing NodeGroups, sorted by name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.fencingNodeGroups").String()).To(MatchJSON(`[
{"name":"ng-notify","mode":"Notify","watchdogTimeout":60},
{"name":"ng-watchdog","mode":"Watchdog","watchdogTimeout":45}
]`))
			var got []fencingNodeGroup
			Expect(json.Unmarshal([]byte(f.ValuesGet("nodeManager.internal.fencingNodeGroups").Raw), &got)).To(Succeed())
			Expect(got[0].Name).To(Equal("ng-notify"))
			Expect(got[1].Name).To(Equal("ng-watchdog"))
		})
	})

	Context("No NodeGroups with fencing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-plain
spec:
  nodeType: Static
`))
			f.RunHook()
		})

		It("writes an empty list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.fencingNodeGroups").String()).To(Equal(`[]`))
		})
	})
})
