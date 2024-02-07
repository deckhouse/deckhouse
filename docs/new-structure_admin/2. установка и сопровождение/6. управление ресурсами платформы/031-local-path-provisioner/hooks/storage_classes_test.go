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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const clusterStorageClasses = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    module: local-path-provisioner
  name: localpath-worker
reclaimPolicy: Retain
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    module: local-path-provisioner
  name: localpath-worker2
reclaimPolicy: Delete
`

var _ = Describe("Local Path Provisioner hooks :: delete SC when reclaimPolicy in CRD is changed ::", func() {
	f := HookExecutionConfigInit(`{"localPathProvisioner":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "LocalPathProvisioner", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterStorageClasses))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("With adding localPathProvisioner crd", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterStorageClasses + `
---
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-worker
spec:
  nodeGroups:
  - worker
  path: "/local"
  reclaimPolicy: "Delete"
`))
			f.RunHook()
		})
		It("Should remove localpath-worker storage class", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.KubernetesGlobalResource("StorageClass", "localpath-worker").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("StorageClass", "localpath-worker2").Exists()).To(BeTrue())
		})
	})
})
