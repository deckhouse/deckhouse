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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const stateLegacyCustomRoles = `
---
# legacy custom role: custom:* name + kind: manage label
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:mycustom:manager
  labels:
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
# legacy custom capability: custom:* name + kind: use label
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:use:capability:mycustom:superresource:view
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
rules:
  - apiGroups: ["deckhouse.io"]
    resources: ["mysuperresources"]
    verbs: ["get", "list", "watch"]
---
# legacy custom role without its own kind label, but with legacy aggregation selectors
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:selectors-only:manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: use
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
rules: []
---
# NOT counted: custom:* name, but unrelated to the RBACv2 role model
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:totally-unrelated
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get"]
---
# NOT counted: new-scheme custom role
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:subsystem:mycustom:manager
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
# NOT counted: legacy labels but a built-in (heritage: deckhouse) object
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:heritage-stray
  labels:
    heritage: deckhouse
    rbac.deckhouse.io/kind: manage
rules: []
---
# NOT counted: legacy kind label but the name does not follow the custom:* convention
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-own-role
  labels:
    rbac.deckhouse.io/kind: manage
rules: []
`

var _ = Describe("User Authz hooks :: discovery legacy custom roles ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	It("value key matches the contract with modules/140-user-authz/requirements", func() {
		// The requirements package duplicates this literal (module requirements packages stay
		// import-free of the hooks package); this assertion and its counterpart in
		// requirements/check_test.go pin both copies to the same string.
		Expect(LegacyRBACv2CustomRolesValueKey).To(Equal("userAuthz:legacyRBACv2CustomRoles"))
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			requirements.RemoveValue("userAuthz:legacyRBACv2CustomRoles")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("saves an empty list to the requirement value", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(LegacyRBACv2CustomRolesValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})

		It("emits no alert metrics, only the group expiration", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_custom_role",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Cluster with a mix of legacy, new-scheme and unrelated roles", func() {
		BeforeEach(func() {
			requirements.RemoveValue("userAuthz:legacyRBACv2CustomRoles")
			f.BindingContexts.Set(f.KubeStateSet(stateLegacyCustomRoles))
			f.RunHook()
		})

		It("saves only the legacy custom roles, sorted", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(LegacyRBACv2CustomRolesValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal([]string{
				"custom:manage:mycustom:manager",
				"custom:manage:selectors-only:manager",
				"custom:use:capability:mycustom:superresource:view",
			}))
		})

		It("emits one alert metric per legacy custom role", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(4))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_custom_role",
				Action: operation.ActionExpireMetrics,
			}))
			for i, name := range []string{
				"custom:manage:mycustom:manager",
				"custom:manage:selectors-only:manager",
				"custom:use:capability:mycustom:superresource:view",
			} {
				Expect(m[i+1]).To(BeEquivalentTo(operation.MetricOperation{
					Name:   "d8_rbacv2_legacy_custom_role",
					Group:  "d8_rbacv2_legacy_custom_role",
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{"name": name},
				}))
			}
		})
	})
})
