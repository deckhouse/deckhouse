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
				capyYandexMachineYAML("m-0", "ng-test", oldAge, nil, true, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
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
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &falseVal, true, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
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
				capyYandexMachineYAML("m-0", "ng-test", youngAge, &trueVal, true, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
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
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
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
				state += capyYandexMachineYAML(name, "ng-test", age, &trueVal, true, false, name)
				state += capyCoreMachineYAML(name, false)
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
				state += capyYandexMachineYAML(name, "ng-big", age, &trueVal, true, false, name)
				state += capyCoreMachineYAML(name, false)
			}
			state += capyYandexMachineYAML("small-0", "ng-small", oldAge, &trueVal, true, false, "small-0")
			state += capyCoreMachineYAML("small-0", false)

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
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
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
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, false, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T8: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine live, but its Machine is already deleting", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false, "m-0") +
				capyCoreMachineYAML("m-0", true)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T9: is not re-selected for deletion", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine and its Machine are both deleting", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, true, "m-0") +
				capyCoreMachineYAML("m-0", true)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T9b: leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine is terminating while its owner Machine is live", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, true, "m-0") +
				capyCoreMachineYAML("m-0", false)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T9c: skips the terminating YandexMachine and keeps the live Machine", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})

	Context("oldest candidate's Machine is deleting, next-oldest is live", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("a", "ng-test", oldAge+time.Hour, &trueVal, true, false, "a") +
				capyCoreMachineYAML("a", true) +
				capyYandexMachineYAML("b", "ng-test", oldAge, &trueVal, true, false, "b") +
				capyCoreMachineYAML("b", false)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T11: the live Machine gets the batch slot", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "b").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "a").Exists()).To(BeTrue())
		})
	})

	Context("20 old candidates, the single oldest has a deleting Machine", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 20, 20)
			for i := 0; i < 20; i++ {
				name := fmt.Sprintf("m-%d", i)
				age := oldAge + time.Duration(i)*time.Minute
				state += capyYandexMachineYAML(name, "ng-test", age, &trueVal, true, false, name)
				state += capyCoreMachineYAML(name, i == 19)
			}
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T12: batch is max(1, live/10) = 1 of 19 live Machines, the oldest live one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-19").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-18").Exists()).To(BeFalse())

			deleted := 0
			for i := 0; i < 20; i++ {
				if !f.KubernetesResource("Machine", "d8-cloud-instance-manager", fmt.Sprintf("m-%d", i)).Exists() {
					deleted++
				}
			}
			Expect(deleted).To(Equal(1))
		})
	})

	Context("old preemptible YandexMachine owned by a Machine absent from the Machine list", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false, "m-gone")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T15: succeeds and deletes nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("YandexMachine without a Machine owner and one owned only by a MachineSet", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false, "") +
				capyCoreMachineYAML("m-0", false) +
				capyMachineSetOwnedYandexMachineYAML("m-1", "ng-test", oldAge) +
				capyCoreMachineYAML("m-1", false)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T16: leaves same-name Machines untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-1").Exists()).To(BeTrue())
		})
	})

	Context("YandexMachine owned by a Machine with a different name, decoy Machine with the YandexMachine's name", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("ym-x", "ng-test", oldAge, &trueVal, true, false, "machine-y") +
				capyCoreMachineYAML("machine-y", false) +
				capyCoreMachineYAML("ym-x", false)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T17: deletes the owner Machine only", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "machine-y").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "ym-x").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("YandexMachine", "d8-cloud-instance-manager", "ym-x").Exists()).To(BeTrue())
		})
	})

	Context("Machine with the owner's name exists only in another namespace", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", oldAge, &trueVal, true, false, "m-0") +
				strings.Replace(capyCoreMachineYAML("m-0", false), "namespace: d8-cloud-instance-manager", "namespace: other", 1)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T4: does not match the foreign-namespace Machine and deletes nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "other", "m-0").Exists()).To(BeTrue())
			Expect(f.PatchCollector.Operations()).To(BeEmpty())
		})
	})

	Context("YandexMachine's node-group label points at a NodeGroup with no status", func() {
		BeforeEach(func() {
			state := capyYandexMachineYAML("m-0", "ng-missing", oldAge, &trueVal, true, false, "m-0") +
				capyCoreMachineYAML("m-0", false)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T10: succeeds and leaves the Machine untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "m-0").Exists()).To(BeTrue())
		})
	})
})

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: capy_delete_preemptible_machines :: Machine API not served ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("infrastructure.cluster.x-k8s.io", "v1alpha1", "YandexMachine", true)

	trueVal := true

	Context("old preemptible YandexMachine while the Machine CRD does not exist", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", 21*time.Hour, &trueVal, true, false, "m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T1: succeeds and keeps the YandexMachine", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(BeEmpty())
		})
	})
})

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: capy_delete_preemptible_machines :: Machine served only as v1beta1 ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("infrastructure.cluster.x-k8s.io", "v1alpha1", "YandexMachine", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Machine", true)

	trueVal := true

	Context("old preemptible YandexMachine while only Machine v1beta1 is served", func() {
		BeforeEach(func() {
			state := capyNodeGroupYAML("ng-test", 10, 10) +
				capyYandexMachineYAML("m-0", "ng-test", 21*time.Hour, &trueVal, true, false, "m-0")
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("T5: succeeds and emits no operations", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(BeEmpty())
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

func capyYandexMachineYAML(name, nodeGroup string, age time.Duration, preemptible *bool, ready, terminating bool, ownerMachine string) string {
	ts := time.Now().UTC().Add(-age).Format(time.RFC3339)

	var meta strings.Builder
	fmt.Fprintf(&meta, "metadata:\n  name: %s\n  namespace: d8-cloud-instance-manager\n  labels:\n    node-group: %s\n  creationTimestamp: %q\n", name, nodeGroup, ts)
	if terminating {
		fmt.Fprintf(&meta, "  deletionTimestamp: %q\n  finalizers:\n  - test.deckhouse.io/hold\n", ts)
	}
	if ownerMachine != "" {
		fmt.Fprintf(&meta, "  ownerReferences:\n  - apiVersion: cluster.x-k8s.io/v1beta2\n    kind: Machine\n    name: %s\n    uid: uid-%s\n    controller: true\n", ownerMachine, ownerMachine)
	}

	spec := "spec: {}\n"
	if preemptible != nil {
		spec = fmt.Sprintf("spec:\n  preemptible: %t\n", *preemptible)
	}

	return fmt.Sprintf("---\napiVersion: infrastructure.cluster.x-k8s.io/v1alpha1\nkind: YandexMachine\n%s%sstatus:\n  ready: %t\n",
		meta.String(), spec, ready)
}

func capyCoreMachineYAML(name string, deleting bool) string {
	var meta strings.Builder
	fmt.Fprintf(&meta, "metadata:\n  name: %s\n  namespace: d8-cloud-instance-manager\n", name)
	if deleting {
		fmt.Fprintf(&meta, "  deletionTimestamp: %q\n  finalizers:\n  - test.deckhouse.io/hold\n", time.Now().UTC().Format(time.RFC3339))
	}

	return fmt.Sprintf("---\napiVersion: cluster.x-k8s.io/v1beta2\nkind: Machine\n%sspec: {}\n", meta.String())
}

func capyMachineSetOwnedYandexMachineYAML(name, nodeGroup string, age time.Duration) string {
	trueVal := true
	ownerRefs := "  ownerReferences:\n  - apiVersion: cluster.x-k8s.io/v1beta2\n    kind: MachineSet\n    name: ms-1\n    uid: uid-ms-1\n    controller: true\n"

	return strings.Replace(capyYandexMachineYAML(name, nodeGroup, age, &trueVal, true, false, ""), "spec:", ownerRefs+"spec:", 1)
}
