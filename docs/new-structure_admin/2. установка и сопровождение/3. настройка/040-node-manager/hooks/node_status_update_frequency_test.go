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

var _ = Describe("Modules :: node-manager :: hooks :: node_status_update_frequency ::", func() {

	const (
		secretNodeMonitorGracePeriodParameter = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-control-plane-manager-control-plane-arguments
  namespace: kube-system
data:
  arguments.json: eyJub2RlU3RhdHVzVXBkYXRlRnJlcXVlbmN5IjogNCwibm9kZU1vbml0b3JQZXJpb2QiOiAyLCJub2RlTW9uaXRvckdyYWNlUGVyaW9kIjogMTV9
`
		secretFailedNodePodEvictionTimeoutParameter = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-control-plane-manager-control-plane-arguments
  namespace: kube-system
data:
  arguments.json: eyJwb2RFdmljdGlvblRpbWVvdXQiOiAxNX0=
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"],"clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; arguments must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeStatusUpdateFrequency").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with nodeMonitorGracePeriod parameter in Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretNodeMonitorGracePeriodParameter))
			f.RunHook()
		})

		It("Hook must not fail; nodeStatusUpdateFrequency must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeStatusUpdateFrequency").String()).To(Equal("4"))
		})
	})

	Context("Cluster with failedNodePodEvictionTimeout parameter in Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretFailedNodePodEvictionTimeoutParameter))
			f.RunHook()
		})

		It("Hook must not fail; nodeStatusUpdateFrequency must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeStatusUpdateFrequency").Exists()).To(BeFalse())
		})
	})

})
