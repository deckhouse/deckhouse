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
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "YandexMachineClass", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("With no proper Machines", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNGsAndMCs()))
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
				"23h10m", "23h30m", "23h40m", "22h",
			)))
			f.RunHook()
		})

		It("All machines after 23h mark should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-2").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-3").Exists()).To(BeTrue())
		})
	})

	Context("With proper machines", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNGsAndMCs(
				"23h10m", "23h30m", "23h40m", "22h",
			)))
			f.RunHook()
		})

		It("All machines after 23h mark should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-2").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-3").Exists()).To(BeTrue())
		})
	})

	Context("With proper machines", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateNGsAndMCs(
				"22h10m", "22h5m", "22h2m", "21h",
			)))
			f.RunHook()
		})

		It("Oldest machine should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "test-3").Exists()).To(BeTrue())
		})
	})
})

func generateNGsAndMCs(durationStrings ...string) string {
	timeNow := time.Now().UTC()

	var offsets []time.Duration
	for _, d := range durationStrings {
		duration, err := time.ParseDuration(d)
		if err != nil {
			panic(err)
		}

		offsets = append(offsets, duration)
	}

	var builder strings.Builder
	builder.WriteString(`---
apiVersion: machine.sapcloud.io/v1alpha1
kind: YandexMachineClass
metadata:
  name: test
  namespace: d8-cloud-instance-manager
spec:
  schedulingPolicy:
    preemptible: true
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: YandexMachineClass
metadata:
  name: not-preemptible
  namespace: d8-cloud-instance-manager
spec:
  schedulingPolicy:
    preemptible: false
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: wrong-instance-class
  namespace: d8-cloud-instance-manager
spec:
  class:
    kind: AWSMachineClass
    name: test
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: terminating
  namespace: d8-cloud-instance-manager
  deletionTimestamp: "1970-01-01T00:00:00Z"
spec:
  class:
    kind: YandexMachineClass
    name: test
`)

	for i, offset := range offsets {
		ts, err := timeNow.Add(-offset).MarshalJSON()
		if err != nil {
			panic(err)
		}

		_, _ = builder.WriteString(fmt.Sprintf(`---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: test-%d
  namespace: d8-cloud-instance-manager
  creationTimestamp: %s
spec:
  class:
    kind: YandexMachineClass
    name: test
`, i, string(ts)))
	}

	return builder.String()
}
