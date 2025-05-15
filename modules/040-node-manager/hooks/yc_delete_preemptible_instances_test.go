/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: preemptibly_delete_preemtible_instances ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "YandexMachineClass", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("With no proper Machines", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNGsAndMCs(10, 10, "")))
			f.RunHook()
		})

		It("Should succeed and no machine should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "wrong-instance-class").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "terminating").Exists()).To(BeTrue())
		})
	})

	Context("With proper machines", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNGsAndMCs(
				7, 7, "", "28h", "22h10m", "22h5m", "22h2m", "21h", "20h", "2h",
			)))
			f.RunHook()
		})

		It("One oldest Machine will be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-4").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-5").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-6").Exists()).To(BeTrue())
		})
	})

	Context("With proper machines", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNGsAndMCs(
				4, 4, "", "22h10m", "22h5m", "22h2m", "21h",
			)))
			f.RunHook()
		})

		It("Oldest Machine should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-3").Exists()).To(BeTrue())
		})

	})

	Context("With 60 Machines older than 24h, one Machine machines younger than 20 hours, and one between 20 and 24 hours", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNMachines(
				60, 60, "test", durationWithCount{count: 60, duration: "30h"},
			)))
			f.RunHook()
		})

		It("15 older than 24 hours Machines and one machine between 20 and 24 hours old should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			for i := 0; i < 54; i++ {
				machineName := fmt.Sprintf("test-0-test-%d", i)
				Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", machineName).Exists()).To(BeTrue())
			}
		})
	})

	Context("With proper machines, but node readiness ratio is below 0.9", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNMachines(
				50, 40, "old", durationWithCount{count: 50, duration: "22h10m"},
			)))
			f.RunHook()
		})

		It("No Machine should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			for i := 0; i < 50; i++ {
				machineName := fmt.Sprintf("old-0-test-%d", i)
				By(fmt.Sprintf("Checking that Machines with name %q still exists", machineName))
				Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", machineName).Exists()).To(BeTrue())
			}
		})
	})
})

type durationWithCount struct {
	duration string
	count    int
}

func generateNMachines(ngNodes, ngReady int, prefix string, machinesCount ...durationWithCount) string {
	var builder strings.Builder

	for i, mc := range machinesCount {
		var durations []string

		for j := 0; j < mc.count; j++ {
			durations = append(durations, mc.duration)
		}

		builder.WriteString(generateNGsAndMCs(ngNodes, ngReady, fmt.Sprintf("%s-%d-", prefix, i), durations...))
	}

	return builder.String()
}

func generateNGsAndMCs(ngNodes, ngReady int, prefix string, durationStrings ...string) string {
	timeNow := time.Now().UTC()

	offsets := make([]time.Duration, 0, len(durationStrings))
	for i, d := range durationStrings {
		duration, err := time.ParseDuration(d)
		if err != nil {
			panic(err)
		}

		duration += time.Duration(i) * time.Millisecond

		offsets = append(offsets, duration)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(strings.ReplaceAll(`---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %stest
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
status:
  nodes: %d
  ready: %d
---
apiVersion: v1
kind: Node
metadata:
  name: %stest
  labels:
    node.deckhouse.io/group: %stest
  creationTimestamp: "1970-01-01T00:00:00Z"
---
apiVersion: v1
kind: Node
metadata:
  name: %snot-preemptible
  labels:
    node.deckhouse.io/group: %stest
  creationTimestamp: "1970-01-01T00:00:00Z"
---
apiVersion: v1
kind: Node
metadata:
  name: %swrong-instance-class
  labels:
    node.deckhouse.io/group: %stest
  creationTimestamp: "1970-01-01T00:00:00Z"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: YandexMachineClass
metadata:
  name: %stest
  namespace: d8-cloud-instance-manager
spec:
  schedulingPolicy:
    preemptible: true
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: YandexMachineClass
metadata:
  name: %snot-preemptible
  namespace: d8-cloud-instance-manager
spec:
  schedulingPolicy:
    preemptible: false
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: %swrong-instance-class
  namespace: d8-cloud-instance-manager
spec:
  class:
    kind: AWSMachineClass
    name: %stest
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: %sterminating
  namespace: d8-cloud-instance-manager
  deletionTimestamp: "1970-01-01T00:00:00Z"
spec:
  class:
    kind: YandexMachineClass
    name: %stest
`, "%s", prefix), ngNodes, ngReady))

	for i, offset := range offsets {
		ts, err := timeNow.Add(-offset).MarshalJSON()
		if err != nil {
			panic(err)
		}

		_, _ = builder.WriteString(fmt.Sprintf(strings.ReplaceAll(`---
apiVersion: v1
kind: Node
metadata:
  name: %s%test-%d
  labels:
    node.deckhouse.io/group: %s%test
  creationTimestamp: %s
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: %s%test-%d
  namespace: d8-cloud-instance-manager
spec:
  class:
    kind: YandexMachineClass
    name: %s%test
`, "%s%", prefix), i, string(ts), i))
	}

	return builder.String()
}
