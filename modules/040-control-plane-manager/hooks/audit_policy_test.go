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

/*

User-stories:
1. There is Secret kube-system/audit-policy with audit-policy.yaml set in data, hook must store it to `controlPlaneManager.internal.auditPolicy`.

*/

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	audit "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: audit_policy ::", func() {
	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = `{"controlPlaneManager":{"apiserver": {}}}`
		secret                 = `
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: %s
`
		configmap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: istiod-service-accounts
  namespace: d8-istio
  labels:
    control-plane-manager.deckhouse.io/extra-audit-policy-config: ""
data:
  basicAuditPolicy: |
    serviceAccounts:
    - system:serviceaccount:d8-istio:istiod-v1x21x6
    - system:serviceaccount:d8-istio:istiod-v1x19x7
`
		policyA = `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
`
		policyB = `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
  omitStages:
    - "RequestReceived"
`
		policyInvalid = `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  somkey: invalidone
`
	)

	policySecret := func(yaml string) string {
		return fmt.Sprintf(secret, base64.StdEncoding.EncodeToString([]byte(yaml)))
	}

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("controlPlaneManager.internal.auditPolicy must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").Exists()).To(BeFalse())
		})
	})

	Context("Invalid policy set", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(policySecret(policyInvalid)))
			f.RunHook()
		})

		It("Must fail on yaml validation", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.GoHookError).Should(MatchError("invalid audit-policy.yaml format: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal object into Go struct field Policy.rules of type []v1.PolicyRule"))
		})
	})

	Context("Cluster started with Secret containing policyA and disabled auditPolicy", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(policySecret(policyA)))
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").Exists()).To(BeFalse())
		})
	})

	Context("Cluster started with Secret containing policyA and not set auditPolicyEnabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(policySecret(policyA)))
			f.ConfigValuesDelete("controlPlaneManager.apiserver.auditPolicyEnabled")
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").Exists()).To(BeFalse())
		})
	})

	Context("Cluster started with Secret containing policyB", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(policySecret(policyA)))
			f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", true)
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must be policyA", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())).To(MatchYAML(policyA))
		})

		Context("Policy changed to policyB", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(policySecret(policyB)))
				f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", true)
				f.RunHook()
			})

			It("controlPlaneManager.internal.auditPolicy must be policyB", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())).To(MatchYAML(policyB))
			})
		})
	})

	Context("Cluster started with basic audit policies", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.basicAuditPolicyEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(configmap))
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must contain proper rules", func() {
			Expect(f).To(ExecuteSuccessfully())
			data, _ := base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())
			var policy audit.Policy
			_ = yaml.UnmarshalStrict(data, &policy)

			istiodServiceAccounts := []string{
				"system:serviceaccount:d8-istio:istiod-v1x21x6",
				"system:serviceaccount:d8-istio:istiod-v1x19x7",
			}

			// All rules, except last nine are dropping rules.
			for i := 0; i < len(policy.Rules)-9; i++ {
				Expect(policy.Rules[i].Level).To(Equal(audit.LevelNone))
			}

			allServiceAccounts := append(auditPolicyBasicServiceAccounts, istiodServiceAccounts...)

			saRule := policy.Rules[len(policy.Rules)-8]
			Expect(saRule.Level).To(Equal(audit.LevelMetadata))
			Expect(saRule.Users).To(Equal(allServiceAccounts))

			namespaceRule := policy.Rules[len(policy.Rules)-6]
			Expect(namespaceRule.Level).To(Equal(audit.LevelMetadata))
			Expect(namespaceRule.Namespaces).To(Equal(auditPolicyBasicNamespaces))

			listRule := policy.Rules[len(policy.Rules)-5]
			Expect(listRule.Level).To(Equal(audit.LevelMetadata))
			Expect(listRule.Namespaces).To(BeEmpty())
		})
	})

	Context("Cluster started with virtualization audit policies", func() {
		BeforeEach(func() {
			f.ValuesSet("global.enabledModules", []string{"virtualization"})
			f.BindingContexts.Set(f.KubeStateSet(configmap))
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must contain proper rules", func() {
			Expect(f).To(ExecuteSuccessfully())
			data, _ := base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())
			var policy audit.Policy
			_ = yaml.UnmarshalStrict(data, &policy)

			// All rules, except first one are metedata level rules.
			for i := 1; i < len(policy.Rules); i++ {
				v := policy.Rules[i]
				Expect(v.Level).To(Equal(audit.LevelMetadata))
			}

			vmopRule := policy.Rules[0]
			Expect(vmopRule.Level).To(Equal(audit.LevelRequestResponse))
		})
	})
})
