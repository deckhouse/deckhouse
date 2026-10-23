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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const perRuleValues = `
admissionPolicyEngine:
  internal:
    ratify:
      webhook:
        ca: test-ca-placeholder
        crt: test-crt-placeholder
        key: test-key-placeholder
    podSecurityStandards:
      enforcementActions:
        - deny
    bootstrapped: true
    webhook:
      ca: test-ca-placeholder
      crt: test-crt-placeholder
      key: test-key-placeholder
    trackedConstraintResources: []
    trackedMutateResources: []
    operationPolicies: []
    securityPolicies: []
  podSecurityStandards:
    defaultPolicy: Baseline
    enforcementAction: Deny
    policies:
      hostPorts:
        knownRanges:
          - max: 35000
            min: 30000
`

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: controller validation per rule", func() {
	render := func(f *Config, module string, internal string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		if module != "" {
			var m map[string]interface{}
			Expect(yaml.Unmarshal([]byte(module), &m)).To(Succeed())
			for k, v := range m {
				f.ValuesSet("admissionPolicyEngine.podSecurityStandards."+k, v)
			}
		}
		if internal != "" {
			var m map[string]interface{}
			Expect(yaml.Unmarshal([]byte(internal), &m)).To(Succeed())
			for k, v := range m {
				f.ValuesSet("admissionPolicyEngine.internal."+k, v)
			}
		}
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	matchesControllers := func(f *Config, kind, name string) bool {
		Expect(f.KubernetesGlobalResource(kind, name).Exists()).To(BeTrue(), "%s %s should exist", kind, name)
		for _, entry := range controllerCheck(f, kind, name).Field("spec.match.kinds").Array() {
			for _, k := range entry.Get("kinds").Array() {
				if k.String() == "Deployment" {
					return true
				}
			}
		}
		return false
	}

	Context("the module excludes Pod Security Standards rules", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, `{controllerValidationExcludedPolicies: [allowHostIPC, requiredDropCapabilities]}`, "")
		})

		It("checks the excluded rule group on Pods only, in every variant of the standard", func() {
			for _, name := range []string{"d8-pod-security-baseline-deny-default", "d8-pod-security-baseline-system", "d8-pod-security-baseline-deny-d8-default"} {
				Expect(matchesControllers(f, "D8HostProcesses", name)).To(BeFalse(), name)
			}
		})

		It("keeps checking controllers against the other rules", func() {
			Expect(matchesControllers(f, "D8HostNetwork", "d8-pod-security-baseline-deny-default")).To(BeTrue())
			Expect(matchesControllers(f, "D8PrivilegedContainer", "d8-pod-security-baseline-deny-default")).To(BeTrue())
		})

		It("skips only the dropped capabilities on controllers in Restricted", func() {
			name := "d8-pod-security-restricted-deny-default"
			Expect(matchesControllers(f, "D8AllowedCapabilities", name)).To(BeTrue())
			Expect(controllerCheck(f, "D8AllowedCapabilities", name).Field("spec.parameters.skipControllerChecks").String()).
				To(MatchJSON(`["requiredDropCapabilities"]`))
		})

		It("does not mark Baseline, which drops no capabilities", func() {
			constraint := controllerCheck(f, "D8AllowedCapabilities", "d8-pod-security-baseline-deny-default")
			Expect(matchesControllers(f, "D8AllowedCapabilities", "d8-pod-security-baseline-deny-default")).To(BeTrue())
			Expect(constraint.Field("spec.parameters.skipControllerChecks").Exists()).To(BeFalse())
		})

		It("publishes the settings in the SecurityPolicy objects of the standards", func() {
			for _, standard := range []string{"baseline", "restricted"} {
				sp := f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-"+standard)
				Expect(sp.Exists()).To(BeTrue())
				Expect(sp.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
				Expect(sp.Field("spec.controllerValidation").String()).
					To(MatchJSON(`{"mode": "Warn", "excludedPolicies": ["allowHostIPC", "requiredDropCapabilities"]}`))
			}
		})
	})

	Context("the published standards", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, "", "")
		})

		It("lists Restricted as Baseline with the overrides applied", func() {
			baseline := f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-baseline").Field("spec.policies")
			restricted := f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-restricted").Field("spec.policies")
			for key, value := range baseline.Map() {
				switch key {
				case "allowPrivilegeEscalation", "allowedCapabilities", "seccompProfiles":
					Expect(restricted.Get(key).Raw).NotTo(MatchJSON(value.Raw), key)
				default:
					Expect(restricted.Get(key).Raw).To(MatchJSON(value.Raw), key)
				}
			}
			Expect(restricted.Get("requiredDropCapabilities").Raw).To(MatchJSON(`["ALL"]`))
			Expect(restricted.Get("runAsUser").Raw).To(MatchJSON(`{"rule": "MustRunAsNonRoot"}`))
		})

		It("renders the constraints from the published rules", func() {
			sp := f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-baseline").Field("spec.policies")
			hostNetwork := f.KubernetesGlobalResource("D8HostNetwork", "d8-pod-security-baseline-deny-default")
			Expect(hostNetwork.Field("spec.parameters.ranges").Raw).To(MatchJSON(sp.Get("allowedHostPorts").Raw))
			capabilities := f.KubernetesGlobalResource("D8AllowedCapabilities", "d8-pod-security-baseline-deny-default")
			Expect(capabilities.Field("spec.parameters.allowedCapabilities").Raw).To(MatchJSON(sp.Get("allowedCapabilities").Raw))
		})

		It("renders in Restricted only the rules it tightens", func() {
			for _, kind := range []string{"D8AllowedCapabilities", "D8AllowPrivilegeEscalation", "D8AllowedVolumeTypes", "D8AllowedUsers", "D8AllowedSeccompProfiles"} {
				Expect(f.KubernetesGlobalResource(kind, "d8-pod-security-restricted-deny-default").Exists()).To(BeTrue(), kind)
			}
			for _, kind := range []string{"D8HostNetwork", "D8HostProcesses", "D8PrivilegedContainer", "D8AppArmor", "D8SeLinux"} {
				Expect(f.KubernetesGlobalResource(kind, "d8-pod-security-restricted-deny-default").Exists()).To(BeFalse(), kind)
			}
		})
	})

	Context("a policy overrides the module", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, `{controllerValidation: false}`, `
securityPolicies:
  - metadata: {name: sp-enforce}
    spec:
      enforcementAction: Warn
      controllerValidation: {mode: Enforce, excludedPolicies: [requiredDropCapabilities]}
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: true
        allowedCapabilities: [NET_BIND_SERVICE]
        requiredDropCapabilities: [ALL]
  - metadata: {name: sp-default}
    spec:
      enforcementAction: Warn
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: true
operationPolicies:
  - metadata: {name: op-excluded}
    spec:
      enforcementAction: Warn
      controllerValidation: {mode: Enforce, excludedPolicies: [requiredResources]}
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowedRepos: [registry.example.com]
        requiredResources: {requests: [cpu]}
  - metadata: {name: op-disabled}
    spec:
      enforcementAction: Warn
      controllerValidation: {mode: Disabled}
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowedRepos: [registry.example.com]
`)
		})

		It("checks controllers when the policy enforces it, whatever the module says", func() {
			Expect(matchesControllers(f, "D8PrivilegedContainer", "sp-enforce")).To(BeTrue())
			Expect(matchesControllers(f, "D8AllowedRepos", "op-excluded")).To(BeTrue())
		})

		It("falls back to the module for a policy without the section", func() {
			Expect(matchesControllers(f, "D8PrivilegedContainer", "sp-default")).To(BeFalse())
			Expect(matchesControllers(f, "D8PrivilegedContainer", "d8-pod-security-baseline-deny-default")).To(BeFalse())
		})

		It("checks an excluded rule on Pods only", func() {
			Expect(matchesControllers(f, "D8RequiredResources", "op-excluded")).To(BeFalse())
		})

		It("checks only Pods when the policy disables it", func() {
			Expect(matchesControllers(f, "D8AllowedRepos", "op-disabled")).To(BeFalse())
		})

		It("splits the capabilities rules of a policy", func() {
			constraint := controllerCheck(f, "D8AllowedCapabilities", "sp-enforce")
			Expect(matchesControllers(f, "D8AllowedCapabilities", "sp-enforce")).To(BeTrue())
			Expect(constraint.Field("spec.parameters.skipControllerChecks").String()).To(MatchJSON(`["requiredDropCapabilities"]`))
		})
	})

	Context("a denying policy spans user and system namespaces", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, "", `
securityPolicies:
  - metadata: {name: sp-split}
    spec:
      enforcementAction: Deny
      controllerValidation: {excludedPolicies: [allowPrivileged]}
      match: {namespaceSelector: {matchNames: [app, d8-foo]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: false
`)
		})

		It("applies the exclusions of the policy to the system variant too", func() {
			for _, name := range []string{"sp-split", "d8-system-default-sp-split"} {
				Expect(matchesControllers(f, "D8PrivilegedContainer", name)).To(BeFalse(), name)
				Expect(matchesControllers(f, "D8AllowPrivilegeEscalation", name)).To(BeTrue(), name)
			}
		})
	})

	Context("module exclusions with several enforcement actions", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, `{controllerValidationExcludedPolicies: [allowPrivilegeEscalation]}`, `
podSecurityStandards:
  enforcementActions: [deny, warn, dryrun]
`)
		})

		It("excludes the rule in every action", func() {
			for _, name := range []string{
				"d8-pod-security-restricted-deny-default", "d8-pod-security-restricted-warn", "d8-pod-security-restricted-dryrun",
				"d8-pod-security-restricted-warn-d8", "d8-pod-security-restricted-system",
			} {
				Expect(matchesControllers(f, "D8AllowPrivilegeEscalation", name)).To(BeFalse(), name)
			}
		})
	})

	Context("a user policy holds the name of a published standard", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, "", `
podSecurityStandards:
  enforcementActions: [deny]
  conflictingPolicyNames: [d8-pod-security-restricted]
`)
		})

		It("leaves that standard unpublished and keeps enforcing it", func() {
			Expect(f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-baseline").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-restricted").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowPrivilegeEscalation", "d8-pod-security-restricted-deny-default").Exists()).To(BeTrue())
		})
	})

	Context("a policy excludes every capabilities rule it sets", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, "", `
securityPolicies:
  - metadata: {name: sp-drop-only}
    spec:
      enforcementAction: Warn
      controllerValidation: {excludedPolicies: [requiredDropCapabilities]}
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: true
        requiredDropCapabilities: [ALL]
`)
		})

		It("checks the constraint on Pods only, without skipControllerChecks", func() {
			constraint := f.KubernetesGlobalResource("D8AllowedCapabilities", "sp-drop-only")
			Expect(matchesControllers(f, "D8AllowedCapabilities", "sp-drop-only")).To(BeFalse())
			Expect(constraint.Field("spec.parameters.skipControllerChecks").Exists()).To(BeFalse())
			Expect(matchesControllers(f, "D8PrivilegedContainer", "sp-drop-only")).To(BeTrue())
		})
	})

	// `controllerValidationEnforcement: Warn`, the default for the transition period.
	Context("controllers are warned about while a policy denies", func() {
		f := SetupHelmConfig(perRuleValues)
		BeforeEach(func() {
			render(f, "", `
podSecurityStandards:
  enforcementActions: [deny, warn, dryrun]
securityPolicies:
  - metadata: {name: sp-deny}
    spec:
      enforcementAction: Deny
      controllerValidation: {excludedPolicies: [requiredDropCapabilities]}
      match: {namespaceSelector: {matchNames: [app, d8-foo]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: true
        allowedCapabilities: [NET_BIND_SERVICE]
        requiredDropCapabilities: [ALL]
  - metadata: {name: sp-warn}
    spec:
      enforcementAction: Warn
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: true
  - metadata: {name: sp-dryrun}
    spec:
      enforcementAction: Dryrun
      match: {namespaceSelector: {matchNames: [app]}}
      policies:
        allowPrivileged: false
        allowPrivilegeEscalation: true
`)
		})

		kindsOf := func(constraint object_store.KubeObject) []string {
			var kinds []string
			for _, entry := range constraint.Field("spec.match.kinds").Array() {
				for _, k := range entry.Get("kinds").Array() {
					kinds = append(kinds, k.String())
				}
			}
			return kinds
		}

		It("denies Pods and warns about controllers in two constraints", func() {
			pods := f.KubernetesGlobalResource("D8PrivilegedContainer", "sp-deny")
			controllers := f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-controllers-sp-deny")
			Expect(pods.Field("spec.enforcementAction").String()).To(Equal("deny"))
			Expect(kindsOf(pods)).To(ConsistOf("Pod"))
			Expect(controllers.Field("spec.enforcementAction").String()).To(Equal("warn"))
			Expect(kindsOf(controllers)).To(ConsistOf("Deployment", "StatefulSet", "DaemonSet", "ReplicationController", "Job", "CronJob"))
			// Both select the same objects, so together they cover what the single constraint did.
			Expect(controllers.Field("spec.match.excludedNamespaces").Raw).To(Equal(pods.Field("spec.match.excludedNamespaces").Raw))
			Expect(controllers.Field("spec.match.namespaces").Raw).To(Equal(pods.Field("spec.match.namespaces").Raw))
			Expect(controllers.Field("spec.match.labelSelector").Raw).To(Equal(pods.Field("spec.match.labelSelector").Raw))
			Expect(controllers.Field("metadata.labels").Raw).To(Equal(pods.Field("metadata.labels").Raw))
		})

		It("skips the excluded checks on the controller constraint only", func() {
			pods := f.KubernetesGlobalResource("D8AllowedCapabilities", "sp-deny")
			controllers := f.KubernetesGlobalResource("D8AllowedCapabilities", "d8-controllers-sp-deny")
			Expect(pods.Field("spec.parameters.skipControllerChecks").Exists()).To(BeFalse())
			Expect(controllers.Field("spec.parameters.skipControllerChecks").String()).To(MatchJSON(`["requiredDropCapabilities"]`))
			Expect(controllers.Field("spec.parameters.allowedCapabilities").Raw).To(Equal(pods.Field("spec.parameters.allowedCapabilities").Raw))
		})

		It("keeps one constraint where the policy does not deny", func() {
			for _, name := range []string{"sp-warn", "sp-dryrun", "d8-system-default-sp-deny"} {
				Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-controllers-"+name).Exists()).To(BeFalse(), name)
				Expect(kindsOf(f.KubernetesGlobalResource("D8PrivilegedContainer", name))).To(ContainElements("Pod", "Deployment"), name)
			}
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "sp-dryrun").Field("spec.enforcementAction").String()).To(Equal("dryrun"))
		})

		It("splits the denying constraints of the Pod Security Standards only", func() {
			for _, name := range []string{"d8-pod-security-baseline-deny-default", "d8-pod-security-baseline-deny-d8-default"} {
				Expect(kindsOf(f.KubernetesGlobalResource("D8HostNetwork", name))).To(ConsistOf("Pod"), name)
				controllers := f.KubernetesGlobalResource("D8HostNetwork", "d8-controllers-"+name)
				Expect(controllers.Field("spec.enforcementAction").String()).To(Equal("warn"), name)
				Expect(controllers.Field("spec.parameters").Raw).To(Equal(f.KubernetesGlobalResource("D8HostNetwork", name).Field("spec.parameters").Raw), name)
			}
			for _, name := range []string{"d8-pod-security-baseline-warn", "d8-pod-security-baseline-dryrun", "d8-pod-security-baseline-system"} {
				Expect(f.KubernetesGlobalResource("D8HostNetwork", "d8-controllers-"+name).Exists()).To(BeFalse(), name)
			}
		})
	})

	Context("controllers are checked with the action of the policy", func() {
		f := SetupHelmConfig(perRuleValues)

		kindsOf := func(constraint object_store.KubeObject) []string {
			var kinds []string
			for _, entry := range constraint.Field("spec.match.kinds").Array() {
				for _, k := range entry.Get("kinds").Array() {
					kinds = append(kinds, k.String())
				}
			}
			return kinds
		}
		policies := `
securityPolicies:
  - metadata: {name: sp-default}
    spec:
      enforcementAction: Deny
      match: {namespaceSelector: {matchNames: [app]}}
      policies: {allowPrivileged: false, allowPrivilegeEscalation: true}
  - metadata: {name: sp-enforce}
    spec:
      enforcementAction: Deny
      controllerValidation: {mode: Enforce}
      match: {namespaceSelector: {matchNames: [app]}}
      policies: {allowPrivileged: false, allowPrivilegeEscalation: true}
  - metadata: {name: sp-warn}
    spec:
      enforcementAction: Deny
      controllerValidation: {mode: Warn}
      match: {namespaceSelector: {matchNames: [app]}}
      policies: {allowPrivileged: false, allowPrivilegeEscalation: true}
`
		denies := func(name string) {
			constraint := f.KubernetesGlobalResource("D8PrivilegedContainer", name)
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-controllers-"+name).Exists()).To(BeFalse(), name)
			Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("deny"), name)
			Expect(kindsOf(constraint)).To(ContainElements("Pod", "Deployment", "CronJob"), name)
		}
		warns := func(name string) {
			Expect(kindsOf(f.KubernetesGlobalResource("D8PrivilegedContainer", name))).To(ConsistOf("Pod"), name)
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-controllers-"+name).Field("spec.enforcementAction").String()).To(Equal("warn"), name)
		}

		Context("when the module sets Policy", func() {
			BeforeEach(func() { render(f, `{controllerValidationEnforcement: Policy}`, policies) })

			It("denies the controllers with the Pod Security Standards and the policies without a mode", func() {
				denies("d8-pod-security-baseline-deny-default")
				denies("sp-default")
				denies("sp-enforce")
			})

			It("keeps warning about the controllers of a policy with mode Warn", func() {
				warns("sp-warn")
			})

			It("publishes the mode in the SecurityPolicy objects of the standards", func() {
				Expect(f.KubernetesGlobalResource("SecurityPolicy", "d8-pod-security-baseline").Field("spec.controllerValidation.mode").String()).To(Equal("Enforce"))
			})
		})

		Context("when the module keeps the default Warn", func() {
			BeforeEach(func() { render(f, "", policies) })

			It("denies the controllers of a policy with mode Enforce only", func() {
				denies("sp-enforce")
				warns("sp-default")
				warns("sp-warn")
				warns("d8-pod-security-baseline-deny-default")
			})
		})
	})
})
