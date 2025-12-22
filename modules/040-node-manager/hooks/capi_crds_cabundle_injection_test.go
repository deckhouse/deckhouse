/*
Copyright 2025 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: inject cabundle to capi crds ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Have a CRD with caBundle not injected and secret + service generated", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Service
metadata:
  name: capi-webhook-service
  namespace: d8-cloud-instance-manager
spec:
  ports:
    - port: 443
      targetPort: webhook-server
  selector:
    app: capi-controller-manager
---
apiVersion: v1
kind: Secret
metadata:
  name: capi-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tQ2FQSUNlcnQK
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: Cluster
    plural: clusters
  scope: Cluster
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          namespace: d8-cloud-instance-manager
          name: capi-webhook-service
          path: /convert
      conversionReviewVersions:
      - v1
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machines.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: Machine
    plural: machines
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinesets.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineSet
    plural: machinesets
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinedeployments.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineDeployment
    plural: machinedeployments
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinepools.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachinePool
    plural: machinepools
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinehealthchecks.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineHealthCheck
    plural: machinehealthchecks
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinedrainrules.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineDrainRule
    plural: machinedrainrules
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: extensionconfigs.runtime.cluster.x-k8s.io
spec:
  group: runtime.cluster.x-k8s.io
  names:
    kind: ExtensionConfig
    plural: extensionconfigs
  scope: Cluster
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should inject caBundle into CRD", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "clusters.cluster.x-k8s.io").Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
		})
	})

	Context("Have a CRD with caBundle already injected and matching", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Service
metadata:
  name: capi-webhook-service
  namespace: d8-cloud-instance-manager
spec:
  ports:
    - port: 443
      targetPort: webhook-server
  selector:
    app: capi-controller-manager
---
apiVersion: v1
kind: Secret
metadata:
  name: capi-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tQ2FQSUNlcnQK
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: Cluster
    plural: clusters
  scope: Cluster
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tQ2FQSUNlcnQK
        service:
          namespace: d8-cloud-instance-manager
          name: capi-webhook-service
          path: /convert
      conversionReviewVersions:
      - v1
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machines.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: Machine
    plural: machines
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinesets.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineSet
    plural: machinesets
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinedeployments.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineDeployment
    plural: machinedeployments
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinepools.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachinePool
    plural: machinepools
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinehealthchecks.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineHealthCheck
    plural: machinehealthchecks
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinedrainrules.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineDrainRule
    plural: machinedrainrules
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: extensionconfigs.runtime.cluster.x-k8s.io
spec:
  group: runtime.cluster.x-k8s.io
  names:
    kind: ExtensionConfig
    plural: extensionconfigs
  scope: Cluster
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should not patch CRD because caBundle is identical", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "clusters.cluster.x-k8s.io").
				Field(`spec.conversion.webhook.clientConfig.caBundle`).String()).
				To(Equal("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tQ2FQSUNlcnQK"))
		})
	})

	Context("Have a CRD with no conversion section and secret + service generated", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Service
metadata:
  name: capi-webhook-service
  namespace: d8-cloud-instance-manager
spec:
  ports:
    - port: 443
      targetPort: webhook-server
  selector:
    app: capi-controller-manager
---
apiVersion: v1
kind: Secret
metadata:
  name: capi-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tQ2FQSUNlcnQK
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machines.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: Machine
    plural: machines
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: Machine
    plural: machines
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinesets.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineSet
    plural: machinesets
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinedeployments.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineDeployment
    plural: machinedeployments
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinepools.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachinePool
    plural: machinepools
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinehealthchecks.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineHealthCheck
    plural: machinehealthchecks
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: machinedrainrules.cluster.x-k8s.io
spec:
  group: cluster.x-k8s.io
  names:
    kind: MachineDrainRule
    plural: machinedrainrules
  scope: Cluster
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: extensionconfigs.runtime.cluster.x-k8s.io
spec:
  group: runtime.cluster.x-k8s.io
  names:
    kind: ExtensionConfig
    plural: extensionconfigs
  scope: Cluster
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should inject a conversion webhook section with caBundle", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "machines.cluster.x-k8s.io").Field(`spec.conversion.webhook.clientConfig.caBundle`).Exists()).To(BeTrue())
			Expect(f.KubernetesResource("CustomResourceDefinition", "", "machines.cluster.x-k8s.io").Field(`spec.conversion.webhook.clientConfig.service.name`).String()).To(Equal("capi-webhook-service"))
		})
	})

	Context("Have secret but no CRDs", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Secret
metadata:
  name: capi-webhook-tls
  namespace: d8-cloud-instance-manager
type: kubernetes.io/tls
data:
  ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tQ2FQSUNlcnQK
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should execute successfully and not patch anything", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
