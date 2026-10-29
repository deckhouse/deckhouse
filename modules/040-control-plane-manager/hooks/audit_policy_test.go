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
	"slices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	audit "k8s.io/apiserver/pkg/apis/audit/v1"
	auditpolicy "k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// auditLevel evaluates the generated audit policy as kube-apiserver does
// and returns the audit level of a non-resource GET request.
// A nil user is a request that failed authentication.
func auditLevel(f *HookExecutionConfig, u user.Info, path string) auditinternal.Level {
	data, err := base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())
	Expect(err).ToNot(HaveOccurred())
	policy, err := auditpolicy.LoadPolicyFromBytes(data)
	Expect(err).ToNot(HaveOccurred())

	attrs := authorizer.AttributesRecord{
		User: u,
		Verb: "get",
		Path: path,
	}
	return auditpolicy.NewPolicyRuleEvaluator(policy).EvaluatePolicyRule(attrs).Level
}

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
metadata: {}
rules:
- level: Metadata
`
		policyB = `
apiVersion: audit.k8s.io/v1
kind: Policy
metadata: {}
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

			var expectPolicy audit.Policy
			extraData := []ConfigMapInfo{
				{
					ServiceAccounts: []string{
						"system:serviceaccount:d8-istio:istiod-v1x21x6",
						"system:serviceaccount:d8-istio:istiod-v1x19x7",
					},
				},
			}

			appendBasicPolicyRules(&expectPolicy, extraData, nil)
			appendVirtualizationPolicyRules(&expectPolicy, nil)
			appendUnauthenticatedRules(&expectPolicy, nil)

			for i, actualRule := range policy.Rules {
				// Note: Equal() is not working here as Rule contains array fields with "omitempty" directive and an empty array is not equal to nil.
				expectedRule := expectPolicy.Rules[i]
				Expect(actualRule.Level).To(Equal(expectedRule.Level), "Level in rule %d %+v should match expected rule %+v", i, actualRule, expectedRule)
				if len(actualRule.Users) > 0 && len(expectedRule.Users) > 0 {
					Expect(actualRule.Users).To(Equal(expectedRule.Users), "Users in rule %d %+v should match expected rule %+v", i, actualRule, expectedRule)
				}
				if len(actualRule.Namespaces) > 0 && len(expectedRule.Namespaces) > 0 {
					Expect(actualRule.Namespaces).To(Equal(expectedRule.Namespaces), "Namespaces in rule %d %+v should match expected rule %+v", i, actualRule, expectedRule)
				}
			}

			hasKubectlLogsRule := false
			for _, rule := range policy.Rules {
				if rule.Level != audit.LevelMetadata {
					continue
				}
				for _, resource := range rule.Resources {
					for _, resourceName := range resource.Resources {
						if resourceName == "pods/log" {
							hasKubectlLogsRule = true
							break
						}
					}
					if hasKubectlLogsRule {
						break
					}
				}
				if hasKubectlLogsRule {
					break
				}
			}
			Expect(hasKubectlLogsRule).To(BeTrue(), "audit policy should contain metadata-level rule for pods/log (kubectl logs)")
		})

		It("must contain explicit kubectl logs audit rule", func() {
			Expect(f).To(ExecuteSuccessfully())
			data, _ := base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())
			var policy audit.Policy
			_ = yaml.UnmarshalStrict(data, &policy)

			found := false
			for _, r := range policy.Rules {
				if r.Level != audit.LevelMetadata {
					continue
				}
				for _, gr := range r.Resources {
					if gr.Group != "" {
						continue
					}
					for _, res := range gr.Resources {
						if res == "pods/log" {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if found {
					break
				}
			}

			Expect(found).To(BeTrue(), "explicit kubectl logs audit rule was not found")
		})

		It("must log kubelet pods/status updates with the request body before the system namespaces rule", func() {
			Expect(f).To(ExecuteSuccessfully())
			data, _ := base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())
			var policy audit.Policy
			_ = yaml.UnmarshalStrict(data, &policy)

			podStatusRule, systemNamespacesRule := -1, -1
			for i, r := range policy.Rules {
				if systemNamespacesRule < 0 && r.Level == audit.LevelMetadata && len(r.Namespaces) > 1 && len(r.Resources) == 0 && len(r.Users) == 0 && len(r.UserGroups) == 0 {
					systemNamespacesRule = i
				}
				if podStatusRule >= 0 || r.Level != audit.LevelRequest || len(r.UserGroups) != 1 || r.UserGroups[0] != "system:nodes" {
					continue
				}
				for _, gr := range r.Resources {
					if gr.Group == "" && len(gr.Resources) == 1 && gr.Resources[0] == "pods/status" {
						podStatusRule = i
					}
				}
			}

			Expect(podStatusRule).To(BeNumerically(">=", 0), "Request-level rule for kubelet pods/status updates was not found")
			Expect(systemNamespacesRule).To(BeNumerically(">=", 0), "system namespaces rule was not found")
			Expect(podStatusRule).To(BeNumerically("<", systemNamespacesRule), "pods/status rule must precede the system namespaces rule")
		})

		It("must log pods/exec requests sent with the get and create verbs", func() {
			Expect(f).To(ExecuteSuccessfully())
			data, _ := base64.StdEncoding.DecodeString(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String())
			var policy audit.Policy
			_ = yaml.UnmarshalStrict(data, &policy)

			var podExecRule *audit.PolicyRule
			for i, r := range policy.Rules {
				if r.Level != audit.LevelRequest || len(r.Users) > 0 || len(r.UserGroups) > 0 || len(r.Namespaces) > 0 {
					continue
				}
				for _, gr := range r.Resources {
					if gr.Group == "" && slices.Contains(gr.Resources, "pods/exec") {
						podExecRule = &policy.Rules[i]
					}
				}
			}

			Expect(podExecRule).ToNot(BeNil(), "Request-level rule for pods/exec was not found")
			Expect(podExecRule.Verbs).To(ContainElements("get", "create"), "pods/exec rule must match both WebSocket (get) and SPDY (create) exec requests")
		})
	})

	Context("Cluster started with basic audit policies and a custom policy that logs all requests", func() {
		var (
			proxyUser     = &user.DefaultInfo{Name: "kubernetes-api-proxy", Groups: []string{"system:authenticated"}}
			adminUser     = &user.DefaultInfo{Name: "admin", Groups: []string{"system:authenticated"}}
			anonymousUser = &user.DefaultInfo{Name: "system:anonymous", Groups: []string{"system:unauthenticated"}}
		)

		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.basicAuditPolicyEnabled", true)
			f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(policySecret(policyA)))
			f.RunHook()
		})

		DescribeTable("audit level of a GET request evaluated by the kube-apiserver policy checker",
			func(u user.Info, path string, expected auditinternal.Level) {
				Expect(f).To(ExecuteSuccessfully())
				Expect(auditLevel(f, u, path)).To(Equal(expected))
			},
			Entry("authenticated /readyz probe is not logged", proxyUser, "/readyz", auditinternal.LevelNone),
			Entry("authenticated /readyz/<check> probe is not logged", proxyUser, "/readyz/etcd", auditinternal.LevelNone),
			Entry("authenticated /livez probe is not logged", adminUser, "/livez", auditinternal.LevelNone),
			Entry("authenticated /healthz probe is not logged", adminUser, "/healthz", auditinternal.LevelNone),
			Entry("anonymous /livez probe is not logged", anonymousUser, "/livez", auditinternal.LevelNone),
			Entry("anonymous /healthz probe is not logged", anonymousUser, "/healthz", auditinternal.LevelNone),
			Entry("anonymous /readyz probe is not logged", anonymousUser, "/readyz", auditinternal.LevelNone),
			Entry("authenticated /version request is logged by the custom policy", adminUser, "/version", auditinternal.LevelMetadata),
			Entry("authenticated /readyzz request is logged by the custom policy", adminUser, "/readyzz", auditinternal.LevelMetadata),
			Entry("anonymous /version request is logged", anonymousUser, "/version", auditinternal.LevelMetadata),
			Entry("/readyz request with failed authentication is logged", nil, "/readyz", auditinternal.LevelMetadata),
			Entry("/livez request with failed authentication is logged", nil, "/livez", auditinternal.LevelMetadata),
		)
	})

	Context("Cluster started with a custom policy that logs all requests and disabled basic audit policies", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.basicAuditPolicyEnabled", false)
			f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(policySecret(policyA)))
			f.RunHook()
		})

		It("must log health probes as the custom policy requires", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(auditLevel(f, &user.DefaultInfo{Name: "kubernetes-api-proxy", Groups: []string{"system:authenticated"}}, "/readyz")).To(Equal(auditinternal.LevelMetadata))
		})
	})

	It("must have RU/EN descriptions for all built-in audit policy rules", func() {
		rules, err := BuiltInAuditPolicyRulesForDocumentation()
		Expect(err).ToNot(HaveOccurred())
		Expect(len(rules)).To(BeNumerically(">", 0))

		var expectedPolicy audit.Policy
		appendBasicPolicyRules(&expectedPolicy, nil, nil)
		appendVirtualizationPolicyRules(&expectedPolicy, nil)
		appendUnauthenticatedRules(&expectedPolicy, nil)

		Expect(len(rules)).To(Equal(len(expectedPolicy.Rules)), "each built-in rule must have a description")

		for i, r := range rules {
			Expect(r.Description.EN).ToNot(BeEmpty(), "missing EN description for rule %d", i)
			Expect(r.Description.RU).ToNot(BeEmpty(), "missing RU description for rule %d", i)
		}
	})

})
