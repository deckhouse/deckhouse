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

var _ = Describe("Modules :: deckhouse :: hooks :: disable default sa token automount ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse":{}}`, `{}`)

	Context("Have a few namespaces with default serviceaccount", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-system
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-system
spec:
  finalizers:
  - kubernetes
status:
  phase: Active
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: d8-system
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: kube-system
  name: kube-system
spec:
  finalizers:
  - kubernetes
status:
  phase: Active
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: kube-system
---
apiVersion: v1
kind: Namespace
metadata:
  labels:
    kubernetes.io/metadata.name: test1
  name: test1
spec:
  finalizers:
  - kubernetes
status:
  phase: Active
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: test1
`
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("Should set automountServiceAccountToken to false on ns with label heritage set to deckhouse", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("SerivceAccount", "d8-system", "default").Field(`automountServiceAccountToken`).Bool()).To(Equal(false))
			Expect(f.KubernetesResource("SerivceAccount", "kube-system", "default").Field(`automountServiceAccountToken`).Bool()).To(Equal(false))
			Expect(f.KubernetesResource("SerivceAccount", "test1", "default").Field(`automountServiceAccountToken`).Exists()).To(BeFalse())
		})
	})

})
