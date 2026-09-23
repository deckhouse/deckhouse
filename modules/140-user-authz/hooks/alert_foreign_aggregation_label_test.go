/*
Copyright 2026 Flant JSC

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

var _ = Describe("User Authz hooks :: alert on a foreign aggregation label ::", func() {
	const roles = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: smuggled
  labels:
    rbac.deckhouse.io/aggregate-to-namespace-as: admin
    rbac.deckhouse.io/aggregate-to-project-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:custom:role:tagged"
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/aggregate-to-system-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:custom:namespace-capability:fine"
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/aggregate-to-namespace-as: viewer
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: "d8:namespace:viewer"
  labels:
    heritage: deckhouse
    rbac.deckhouse.io/kind: role
    rbac.deckhouse.io/aggregate-to-namespace-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: plain
rules: []
`
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, "")

	series := func() map[string]string {
		out := map[string]string{}
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == "d8_user_authz_foreign_aggregation_label" && m.Value != nil {
				out[m.Labels["name"]+"/"+m.Labels["lineage"]] = m.Labels["kind"]
			}
		}
		return out
	}

	Context("roles of every shape are present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(roles))
			f.RunHook()
		})
		It("flags only the roles that carry the label without being a custom capability or a platform role", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(series()).To(Equal(map[string]string{
				"smuggled/namespace":           "",
				"smuggled/project":             "",
				"d8:custom:role:tagged/system": "custom-role",
			}))
		})
	})

	Context("no ClusterRole carries the label outside the contract", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("exports nothing and only expires the group", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(series()).To(BeEmpty())
		})
	})
})
