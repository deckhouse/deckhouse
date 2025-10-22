// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. There is CM kube-system/d8-cluster-uuid with cluster uuid. Hook must store it to `global.discovery.clusterUUID`.
2. There isn't CM kube-system/d8-cluster-uuid. Hook must generate new UUID, store it to `global.discovery.clusterUUID` and create CM with it.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cluster_uuid ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const cmUUID = "2528b7ff-a5eb-48d1-b0b0-4c87628284de"

	const (
		stateCM = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-uuid
  namespace: kube-system
data:
  cluster-uuid: ` + cmUUID + "\n"
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("`global.discovery.clusterUUID` must be generated and config map must be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			newUUID := f.ValuesGet("global.discovery.clusterUUID").String()
			Expect(len(newUUID)).To(Equal(36))

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-uuid")
			Expect(cm.Field("data.cluster-uuid").String()).To(Equal(newUUID))
		})
	})

	Context("CM d8-cluster-uuid exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCM))
			f.RunHook()
		})

		It("'global.discovery.clusterUUID' must be set from config map", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterUUID").String()).To(Equal(cmUUID))
		})

		Context("CM d8-cluster-uuid deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("Must create config map with cluster uuid from values", func() {
				Expect(f).To(ExecuteSuccessfully())
				cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-uuid")
				Expect(cm.Field("data.cluster-uuid").String()).To(Equal(cmUUID))
			})
		})
	})
})
