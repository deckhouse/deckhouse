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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/shared"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: set_node_group_status_engine ::", func() {
	const (
		ngs = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-e
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 3
    maxPerZone: 3
    classReference:
      kind: D8TestInstanceClass
      name: cap
    zones: [a,b]
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-p
spec:
  nodeType: CloudPermanent
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-cs
spec:
  nodeType: CloudStatic
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-s
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-sc
spec:
  nodeType: Static
  staticInstances:
    count: 0
    labelSelector:
      matchLabels:
        node-group: worker
`
	)
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}, "instancePrefix": "test"}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	assertMigrationFinishedCMCreated := func(f *HookExecutionConfig) {
		cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-node-group-engine-migration")
		Expect(cm.Exists()).To(BeTrue())
	}

	assertSetValidEngine := func(f *HookExecutionConfig, cloudEphemeralDefaultEngine string) {
		Expect(f.KubernetesGlobalResource("NodeGroup", "test-e").Field("status.engine").Value()).To(Equal(cloudEphemeralDefaultEngine))
		Expect(f.KubernetesGlobalResource("NodeGroup", "test-p").Field("status.engine").Value()).To(Equal("None"))
		Expect(f.KubernetesGlobalResource("NodeGroup", "test-cs").Field("status.engine").Value()).To(Equal("None"))
		Expect(f.KubernetesGlobalResource("NodeGroup", "test-s").Field("status.engine").Value()).To(Equal("None"))
		Expect(f.KubernetesGlobalResource("NodeGroup", "test-sc").Field("status.engine").Value()).To(Equal("CAPI"))
	}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Migration CM should created", func() {
			assertMigrationFinishedCMCreated(f)
		})
	})

	Context("Cluster with node groups", func() {
		Context("For CAPI only cloud providers", func() {
			for _, module := range shared.ProvidersWithCAPIOnly {
				Context(fmt.Sprintf("Cloud provider for %s", module), func() {
					BeforeEach(func() {
						f.ValuesSet("global.enabledModules", []string{module})
						f.BindingContexts.Set(f.KubeStateSet(ngs))
						f.BindingContexts.Set(f.GenerateBeforeHelmContext())
						f.RunHook()
					})

					It("Hook must not fail", func() {
						Expect(f).To(ExecuteSuccessfully())
					})

					It("Set engine correct", func() {
						assertSetValidEngine(f, "CAPI")
					})

					It("Migration CM should created", func() {
						assertMigrationFinishedCMCreated(f)
					})
				})

			}
		})

		Context("For CAPI and MCM cloud providers", func() {
			for _, module := range shared.ProvidersWithCAPIAnsMCM {
				Context(fmt.Sprintf("Cloud provider for %s", module), func() {
					BeforeEach(func() {
						f.ValuesSet("global.enabledModules", []string{module})
						f.BindingContexts.Set(f.KubeStateSet(ngs))
						f.BindingContexts.Set(f.GenerateBeforeHelmContext())
						f.RunHook()
					})

					It("Hook must not fail", func() {
						Expect(f).To(ExecuteSuccessfully())
					})

					It("Set engine correct", func() {
						assertSetValidEngine(f, "MCM")
					})

					It("Migration CM should created", func() {
						assertMigrationFinishedCMCreated(f)
					})
				})
			}
		})

		Context("For MCM cloud providers", func() {
			for _, module := range []string{"cloud-provider-aws", "cloud-provider-azure", "cloud-provider-gcp", "cloud-provider-yandex", "cloud-provider-vsphere"} {
				Context(fmt.Sprintf("Cloud provider for %s", module), func() {
					BeforeEach(func() {
						f.ValuesSet("global.enabledModules", []string{module})
						f.BindingContexts.Set(f.KubeStateSet(ngs))
						f.BindingContexts.Set(f.GenerateBeforeHelmContext())
						f.RunHook()
					})

					It("Hook must not fail", func() {
						Expect(f).To(ExecuteSuccessfully())
					})

					It("Set engine correct", func() {
						assertSetValidEngine(f, "MCM")
					})

					It("Migration CM should created", func() {
						assertMigrationFinishedCMCreated(f)
					})
				})
			}
		})
	})

	Context("Cluster has migration cm", func() {
		const ng = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-e
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 3
    maxPerZone: 3
    classReference:
      kind: D8TestInstanceClass
      name: cap
    zones: [a,b]
status:
  engine: MCM
`
		BeforeEach(func() {
			f.ValuesSet("global.enabledModules", []string{"cloud-provider-openstack"})
			f.BindingContexts.Set(f.KubeStateSet(ngs))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Migration CM should created", func() {
			Expect(f.KubernetesGlobalResource("NodeGroup", "test-e").Field("status.engine").Value()).To(Equal("MCM"))
		})

		It("Migration cm stay in the created", func() {
			assertMigrationFinishedCMCreated(f)
		})
	})
})
