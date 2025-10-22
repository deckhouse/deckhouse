/*
Copyright 2021 Flant JSC

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
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Local Path Provisioner hooks :: get localpathprovisioner crds ::", func() {
	f := HookExecutionConfigInit(`{"localPathProvisioner":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "LocalPathProvisioner", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("With adding localPathProvisioner object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: local1
spec:
  nodeGroups:
  - master
  path: "/local"
  reclaimPolicy: "Retain"
`))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("localPathProvisioner.internal.localPathProvisioners").String()).To(MatchJSON(`
[{
    "name": "local1",
    "spec": {
      "nodeGroups": ["master"],
      "path": "/local",
      "reclaimPolicy": "Retain"
    }
}]`))
			})

			Context("With deleting localPathProvisioner object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})
				It("Internal value should be an empty JSON array", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("localPathProvisioner.internal.localPathProvisioners").String()).To(MatchJSON("[]"))
				})
			})
			Context("With updating localPathProvisioner object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: local1
spec:
  nodeGroups:
  - worker
  - system
  path: "/opt/local-path-provisioner"
  reclaimPolicy: "Delete"
`))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("localPathProvisioner.internal.localPathProvisioners").String()).To(MatchJSON(`
[{
    "name": "local1",
    "spec": {
      "nodeGroups": ["worker", "system"],
      "path": "/opt/local-path-provisioner",
      "reclaimPolicy": "Delete"
    }
}]`))
				})
			})
		})
	})

	Context("Many localPathProvisioner objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: local1
spec:
  nodeGroups:
  - master
  - worker
  path: "/opt/local-path-provisioner"
  reclaimPolicy: "Delete"
---
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: local2
spec:
  path: "/local"
  reclaimPolicy: "Retain"
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f.ValuesGet("localPathProvisioner.internal.localPathProvisioners").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "local1",
    "spec": {
      "nodeGroups": ["master", "worker"],
      "path": "/opt/local-path-provisioner",
      "reclaimPolicy": "Delete"
    }
  },
  {
    "name": "local2",
    "spec": {
      "path": "/local",
      "reclaimPolicy": "Retain"
    }
  }
]`))
		})
	})

})
