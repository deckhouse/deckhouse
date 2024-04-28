/*
Copyright 2024 Flant JSC

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

package migrate

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: adopt_node_manager_resources ::", func() {
	f := HookExecutionConfigInit(`{"deckhouse": { "internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has ns in the deckhouse helm release", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    helm.sh/resource-policy: keep
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-cloud-instance-manager
    module: node-manager
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-cloud-instance-manager
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager").ToYaml()).To(MatchYAML(`
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    helm.sh/resource-policy: keep
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-cloud-instance-manager
    module: node-manager
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-cloud-instance-manager
`))
		})
	})

	Context("Cluster has ns in the node-manager helm release", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    meta.helm.sh/release-name: node-manager
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-cloud-instance-manager
    module: node-manager
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-cloud-instance-manager
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Namespace", "d8-cloud-instance-manager").ToYaml()).To(MatchYAML(`
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    helm.sh/resource-policy: keep
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-cloud-instance-manager
    module: node-manager
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-cloud-instance-manager
`))
		})
	})

	Context("Cluster has cm in the deckhouse helm release", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  name: kube-rbac-proxy-ca.crt
  namespace: d8-cloud-instance-manager
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "kube-rbac-proxy-ca.crt").ToYaml()).To(MatchYAML(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  name: kube-rbac-proxy-ca.crt
  namespace: d8-cloud-instance-manager
`))
		})
	})

	Context("Cluster has ns in the node-manager helm release", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: node-manager
    meta.helm.sh/release-namespace: d8-system
  name: kube-rbac-proxy-ca.crt
  namespace: d8-cloud-instance-manager
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "kube-rbac-proxy-ca.crt").ToYaml()).To(MatchYAML(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  name: kube-rbac-proxy-ca.crt
  namespace: d8-cloud-instance-manager
`))
		})
	})

})
