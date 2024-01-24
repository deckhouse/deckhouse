/*
Copyright 2023 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: cluster_api_deckhouse_control_plane ::", func() {
	const (
		controlPlane = `
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: DeckhouseControlPlane
metadata:
  name: control-plane
  namespace: d8-cloud-instance-manager
`
		controlPlaneWithStatus = `
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: DeckhouseControlPlane
metadata:
  name: control-plane
  namespace: d8-cloud-instance-manager
status:
  externalManagedControlPlane: true
  initialized: true
  ready: true
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("infrastructure.cluster.x-k8s.io", "v1alpha1", "DeckhouseControlPlane", false)

	Context("DeckhouseControlPlane with status field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(controlPlane, 1))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("DeckhouseControlPlane", "d8-cloud-instance-manager", "control-plane").ToYaml()).To(MatchYAML(controlPlaneWithStatus))
		})
	})
})
