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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: capy_delete_preemptible_machines ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("infrastructure.cluster.x-k8s.io", "v1alpha1", "YandexMachine", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta2", "Machine", true)

	const (
		oldAge   = 21 * time.Hour
		youngAge = 10 * time.Hour
	)

	trueVal := true
	falseVal := false

	Context("YandexMachine with spec.preemptible absent", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, nil, true, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T1: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine with spec.preemptible explicit false", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &falseVal, true, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T2: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine preemptible and ready, but younger than the threshold", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", youngAge, &trueVal, true, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T3: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine preemptible, old, ready, healthy NodeGroup ratio", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T4: deletes the owning Machine, not the YandexMachine", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("YandexMachine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("25 eligible old YandexMachines in one NodeGroup", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 25, 25)
			for i := 0; i < 25; i++ {
				name := fmt.Sprintf("m-%d", i)
				// space out ages so the sort order (oldest-first) is deterministic.
				age := oldAge + time.Duration(i)*time.Minute
				state += capyYandexMachineYAML(name, "ng-test", age, &trueVal, true, false)
				state += capyCoreMachineYAML(name)
			}
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T5: deletes exactly max(1, N/10) Machines, the oldest first", func() {
			Expect(f).To(ExecuteSuccessfully())

			// oldest ones (highest index -> largest age offset) must be gone
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-24").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-23").Exists()).To(BeFalse())

			deleted := 0
			for i := 0; i < 25; i++ {
				name := fmt.Sprintf("m-%d", i)
				if !f.KubernetesResource("Machine", "d8-cloud-instance-manager", name).Exists() {
					deleted++
				}
			}
			Expect(deleted).To(Equal(2))
		})
	})

	Context("two NodeGroups: one with 20 eligible candidates, one with a single candidate", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-big", 20, 20) + capyNodeGroupYAML("ng-small", 1, 1)
			for i := 0; i < 20; i++ {
				name := fmt.Sprintf("big-%d", i)
				age := oldAge + time.Duration(i)*time.Minute
				state += capyYandexMachineYAML(name, "ng-big", age, &trueVal, true, false)
				state += capyCoreMachineYAML(name)
			}
			state += capyYandexMachineYAML("small-0", "ng-small", oldAge, &trueVal, true, false)
			state += capyCoreMachineYAML("small-0")

			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T6: the single-candidate NodeGroup's Machine is deleted in the same run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "small-0").Exists()).To(BeFalse())
		})
	})

	Context("YandexMachine preemptible, old, ready, but NodeGroup readiness ratio below 0.9", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 8) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T7: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine preemptible, old, but status.ready is false", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, false, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T8: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine preemptible, old, ready, but already Terminating", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, true) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T9: is not re-selected for deletion", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine's node-group label points at a NodeGroup with no status", func() {
		BeforeEach(func() {
			state := capyYandexMachineYAML("m-0", "ng-missing", oldAge, &trueVal, true, false) +
				capyCoreMachineYAML("m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T10: succeeds and leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})
})

func capyNodeGroupYAML(name string, nodes, ready int64) string {
	return fmt.Sprintf(`---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %s
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
status:
  nodes: %d
  ready: %d
`, name, nodes, ready)
}

func capyYandexMachineYAML(name, nodeGroup string, age time.Duration, preemptible *bool, ready, terminating bool) string {
	ts := time.Now().UTC().Add(-age).Format(time.RFC3339)

	var meta strings.Builder
	fmt.Fprintf(&meta, "metadata:\n  name: %s\n  namespace: d8-cloud-instance-manager\n  labels:\n    node-group: %s\n  creationTimestamp: %q\n", name, nodeGroup, ts)
	if terminating {
		fmt.Fprintf(&meta, "  deletionTimestamp: %q\n", ts)
	}

	spec := "spec: {}\n"
	if preemptible != nil {
		spec = fmt.Sprintf("spec:\n  preemptible: %t\n", *preemptible)
	}

	return fmt.Sprintf("---\napiVersion: infrastructure.cluster.x-k8s.io/v1alpha1\nkind: YandexMachine\n%s%sstatus:\n  ready: %t\n",
		meta.String(), spec, ready)
}

func capyCoreMachineYAML(name string) string {
	return fmt.Sprintf(`---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Machine
metadata:
  name: %s
  namespace: d8-cloud-instance-manager
spec: {}
`, name)
}
