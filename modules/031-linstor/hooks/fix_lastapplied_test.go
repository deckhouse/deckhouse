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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: linstor :: hooks :: fix_lastapplied ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Linstor deployments created with affinity on master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: linstor-controller
  namespace: d8-linstor
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/master
                operator: In
                values:
                - ""
      tolerations:
      - key: node-role.kubernetes.io/master
      - key: dedicated.deckhouse.io
        operator: Exists
      - key: dedicated
        operator: Exists
      - key: ToBeDeletedByClusterAutoscaler
      - key: node.kubernetes.io/not-ready
      - key: node.kubernetes.io/out-of-disk
      - key: node.kubernetes.io/memory-pressure
      - key: node.kubernetes.io/disk-pressure
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: linstor-csi-controller
  namespace: d8-linstor
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/master
                operator: In
                values:
                - ""
      tolerations:
      - key: node-role.kubernetes.io/master
      - key: dedicated.deckhouse.io
        operator: Exists
      - key: dedicated
        operator: Exists
      - key: ToBeDeletedByClusterAutoscaler
      - key: node.kubernetes.io/not-ready
      - key: node.kubernetes.io/out-of-disk
      - key: node.kubernetes.io/memory-pressure
      - key: node.kubernetes.io/disk-pressure
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Must delete linstor-controller deployment with nodeAffinity and tolerations set for master", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Deployment", "d8-linstor", "linstor-controller").Exists()).To(BeFalse())
		})

		It("Must delete linstor-csi-controller deployment with nodeAffinity and tolerations set for master", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Deployment", "d8-linstor", "linstor-csi-controller").Exists()).To(BeFalse())
		})
	})

	Context("Linstor deployments created with affinity on system", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: linstor-controller
  namespace: d8-linstor
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/system
                operator: In
                values:
                - ""
      tolerations:
      - key: node-role.kubernetes.io/system
      - key: dedicated.deckhouse.io
        operator: Exists
      - key: dedicated
        operator: Exists
      - key: ToBeDeletedByClusterAutoscaler
      - key: node.kubernetes.io/not-ready
      - key: node.kubernetes.io/out-of-disk
      - key: node.kubernetes.io/memory-pressure
      - key: node.kubernetes.io/disk-pressure
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: linstor-csi-controller
  namespace: d8-linstor
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/system
                operator: In
                values:
                - ""
      tolerations:
      - key: node-role.kubernetes.io/system
      - key: dedicated.deckhouse.io
        operator: Exists
      - key: dedicated
        operator: Exists
      - key: ToBeDeletedByClusterAutoscaler
      - key: node.kubernetes.io/not-ready
      - key: node.kubernetes.io/out-of-disk
      - key: node.kubernetes.io/memory-pressure
      - key: node.kubernetes.io/disk-pressure
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Must keep linstor-controller deployment with nodeAffinity and tolerations set for system", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Deployment", "d8-linstor", "linstor-controller").Exists()).To(BeTrue())
		})

		It("Must keep linstor-csi-controller deployment with nodeAffinity and tolerations set for system", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Deployment", "d8-linstor", "linstor-csi-controller").Exists()).To(BeTrue())
		})
	})

})
