/*
Copyright 2022 Flant JSC

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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	nsName = "d8-admission-policy-engine"

	globalValues = `
deckhouseVersion: test
deckhouseEdition: CSE
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.32.0"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`
)

func templateLibsDir() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "charts", "constraint-templates", "templates", "libs"))
}

func disableTemplateLibRegoTests() ([][2]string, error) {
	libsDir := templateLibsDir()
	entries, err := os.ReadDir(libsDir)
	if err != nil {
		return nil, err
	}
	moved := make([][2]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, "_test.rego") {
			continue
		}
		src := filepath.Join(libsDir, name)
		dst := src + ".template-tests.disabled"
		if err := os.Rename(src, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(moved[i][1], moved[i][0])
			}
			return nil, err
		}
		moved = append(moved, [2]string{src, dst})
	}
	return moved, nil
}

func restoreTemplateLibRegoTests(moved [][2]string) error {
	for i := len(moved) - 1; i >= 0; i-- {
		if err := os.Rename(moved[i][1], moved[i][0]); err != nil {
			return err
		}
	}
	return nil
}

var _ = Describe("Module :: admissionPolicyEngine :: helm template ::", func() {
	f := SetupHelmConfig(`{"admissionPolicyEngine": {"podSecurityStandards": {}, "internal": {"ratify": {"imageReferences": [{"reference": "ghcr.io/*", "publicKeys": ["someKey2"]}], "webhook": {"key": "YjY0ZW5jX3N0cmluZwo=", "crt": "YjY0ZW5jX3N0cmluZwo=" , "ca": "YjY0ZW5jX3N0cmluZwo="}}, "podSecurityStandards": {"enforcementActions": ["deny"]}, "operationPolicies": [
	{
		"metadata": {
			"name": "foo"
		},
		"spec": {
			"enforcementAction": "Deny",
			"match": {
				"labelSelector": {
					"matchLabels": {
						"operation-policy.deckhouse.io/enabled": "true"
					}
				},
				"namespaceSelector": {
					"excludeNames": [
						"some-ns"
					],
					"labelSelector": {
						"matchLabels": {
							"operation-policy.deckhouse.io/enabled": "true"
						}
					},
					"matchNames": [
						"default"
					]
				}
			},
			"policies": {
				"allowedRepos": [
					"foo"
				]
			}
		}
    	}
],
        "securityPolicies": [
	{
		"metadata": {
			"name": "foo"
		},
		"spec": {
			"enforcementAction": "Deny",
			"match": {
				"namespaceSelector": {
					"labelSelector": {
						"matchLabels": {
								"security-policy.deckhouse.io/enabled": "true"
						}
					}
				},
				"labelSelector": {}
			},
			"policies": {
				"allowedAppArmor": [
					"runtime/default"
				],
				"allowedFlexVolumes": [
					{
						"driver": "vmware"
					}
				],
				"allowedHostPaths": [
					{
						"pathPrefix": "/dev",
						"readOnly": true
					}
				],
				"allowedHostPorts": [
					{
						"max": 100,
						"min": 10
					}
				],
				"allowedUnsafeSysctls": [
					"*"
				],
				"allowHostIPC": true,
				"allowHostNetwork": false,
				"allowHostPID": false,
				"allowPrivileged": false,
				"allowPrivilegeEscalation": false,
				"automountServiceAccountToken": true,
				"forbiddenSysctls": [
					"user/example"
				],
				"readOnlyRootFilesystem": true,
				"requiredDropCapabilities": [
					"ALL"
				],
				"runAsUser": {
					"ranges": [
						{
							"max": 500,
							"min": 300
						}
					],
					"rule": "MustRunAs"
				},
				"seccompProfiles": {
					"allowedLocalhostFiles": [
						"*"
					],
					"allowedProfiles": [
						"RuntimeDefault",
						"Localhost"
					]
				},
				"seLinux": [
					{
						"role": "role",
						"user": "user"
					},
					{
						"level": "level",
						"type": "type"
					}
				],
				"supplementalGroups": {
					"ranges": [
						{
							"max": 1000,
							"min": 500
						}
					],
					"rule": "MustRunAs"
				},
				"verifyImageSignatures": [
					{
						"dockerCfg": "zxc=",
						"reference": "ghcr.io/*",
						"publicKeys": ["someKey2"]
					}
				]
			}
		}
	}
],
	trackedConstraintResources: [{"apiGroups":[""],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]},{"apiGroups": [""],"resources": ["pods/exec","pods/attach"],"operations": ["CONNECT"]}],
	trackedMutateResources: [{"apiGroups":[""],"resources":["pods"]}],
	webhook: {ca: YjY0ZW5jX3N0cmluZwo=, crt: YjY0ZW5jX3N0cmluZwo=, key: YjY0ZW5jX3N0cmluZwo=}}}}`)

	checkVWC := func(f *Config, webhooksCount int, rules ...string) {
		vw := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config")
		Expect(vw.Exists()).To(BeTrue())
		Expect(vw.Field("webhooks").Array()).To(HaveLen(webhooksCount))
		for i := 0; i < webhooksCount; i++ {
			Expect(vw.Field(fmt.Sprintf("webhooks.%d.rules", i)).String()).To(MatchJSON(rules[i]))
		}
	}

	var movedTemplateRegoTests [][2]string

	BeforeSuite(func() {
		err := os.Symlink("/deckhouse/ee/se-plus/modules/015-admission-policy-engine/templates/ratify", "/deckhouse/modules/015-admission-policy-engine/templates/ratify")
		Expect(err).ShouldNot(HaveOccurred())

		movedTemplateRegoTests, err = disableTemplateLibRegoTests()
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := restoreTemplateLibRegoTests(movedTemplateRegoTests)
		Expect(err).ShouldNot(HaveOccurred())

		err = os.Remove("/deckhouse/modules/015-admission-policy-engine/templates/ratify")
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, "admission-policy-engine")
			dp := f.KubernetesResource("Deployment", nsName, "gatekeeper-controller-manager")
			tp := f.KubernetesResource("StatefulSet", nsName, "trivy-provider")
			r := f.KubernetesResource("Deployment", nsName, "ratify")
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(tp.Exists()).To(BeFalse())
			Expect(r.Exists()).To(BeFalse())

			tpSvc := f.KubernetesResource("Service", nsName, "trivy-provider")
			Expect(tpSvc.Exists()).To(BeFalse())
			ratifySvc := f.KubernetesResource("Service", nsName, "ratify")
			Expect(ratifySvc.Exists()).To(BeFalse())

			vw := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config")
			Expect(vw.Exists()).To(BeFalse())
		})
	})

	Context("Cluster with deckhouse on master node with bootstrapped module", func() {
		BeforeEach(func() {
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.trackedConstraintResources", `[]`)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Renders ValidatingWebhookConfiguration with deny-exec-heritage webhook only", func() {
			denyExecHeritageRules := `[{"apiGroups":[""],"apiVersions":["*"],"operations":["CONNECT"],"resources":["pods/exec","pods/attach"]}]`
			checkVWC(f, 1, denyExecHeritageRules)
		})
	})

	Context("Cluster with deckhouse on master node with bootstrapped module and trackedConstraintResources", func() {
		BeforeEach(func() {
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.trackedConstraintResources", `[{"apiGroups":[""],"resources":["pods"]}]`)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Renders ValidatingWebhookConfiguration with main, the two system-namespace webhooks and deny-exec-heritage", func() {
			mainRules := `[{"apiGroups":[""],"apiVersions":["*"],"operations":["CREATE","UPDATE","DELETE"],"resources":["pods"]},{"apiGroups":["rbac.authorization.k8s.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE","DELETE"],"resources":["roles","rolebindings"]},{"apiGroups":["constraints.gatekeeper.sh"],"apiVersions":["*"],"operations":["CREATE","UPDATE","DELETE"],"resources":["*"],"scope":"*"},{"apiGroups":[""],"apiVersions":["*"],"resources":["pods/exec","pods/attach"],"operations":["CONNECT"]},{"apiGroups":[""],"apiVersions":["*"],"resources":["pods/ephemeralcontainers"],"operations":["UPDATE"]}]`
			// The system-namespaces webhook leaves out exec and attach: deny-exec-heritage already routes them.
			systemNamespacesRules := `[{"apiGroups":[""],"apiVersions":["*"],"operations":["CREATE","UPDATE","DELETE"],"resources":["pods"]},{"apiGroups":["rbac.authorization.k8s.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE","DELETE"],"resources":["roles","rolebindings"]},{"apiGroups":["constraints.gatekeeper.sh"],"apiVersions":["*"],"operations":["CREATE","UPDATE","DELETE"],"resources":["*"],"scope":"*"},{"apiGroups":[""],"apiVersions":["*"],"resources":["pods/ephemeralcontainers"],"operations":["UPDATE"]}]`
			denyExecHeritageRules := `[{"apiGroups":[""],"apiVersions":["*"],"operations":["CONNECT"],"resources":["pods/exec","pods/attach"]}]`
			checkVWC(f, 4, mainRules, systemNamespacesRules, systemNamespacesRules, denyExecHeritageRules)

			vw := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config")
			systemNamespacesExpr := `has(request.namespace) && (request.namespace.startsWith("d8-") || request.namespace.startsWith("kube-"))`

			// The main webhook must leave the system namespaces to the two below it.
			Expect(vw.Field("webhooks.0.namespaceSelector").Exists()).To(BeFalse())
			Expect(vw.Field("webhooks.0.matchConditions.0.expression").String()).To(Equal("!(" + systemNamespacesExpr + ")"))
			Expect(vw.Field("webhooks.0.failurePolicy").String()).To(Equal("Fail"))

			Expect(vw.Field("webhooks.1.name").String()).To(Equal("system-namespaces.admission-policy-engine.deckhouse.io"))
			Expect(vw.Field("webhooks.1.matchConditions.0.expression").String()).To(Equal(systemNamespacesExpr))
			Expect(vw.Field("webhooks.2.name").String()).To(Equal("system-namespaces-enforce.admission-policy-engine.deckhouse.io"))
			Expect(vw.Field("webhooks.2.matchConditions.0.expression").String()).To(Equal(systemNamespacesExpr))

			// Neither system-namespace webhook may block a workload while Gatekeeper is unavailable.
			Expect(vw.Field("webhooks.1.failurePolicy").String()).To(Equal("Ignore"))
			Expect(vw.Field("webhooks.2.failurePolicy").String()).To(Equal("Ignore"))

			// Both select on the label VALUE, exactly as the constraints they serve do, so a
			// namespace labeled with anything but "true" cannot fall between them.
			Expect(vw.Field("webhooks.1.namespaceSelector.matchExpressions").String()).To(MatchJSON(
				`[{"key":"security.deckhouse.io/enable-security-policy-check","operator":"NotIn","values":["true"]}]`))
			Expect(vw.Field("webhooks.2.namespaceSelector.matchExpressions").String()).To(MatchJSON(
				`[{"key":"security.deckhouse.io/enable-security-policy-check","operator":"In","values":["true"]}]`))

			// Exec and attach stay the one deliberate Fail path in system namespaces.
			Expect(vw.Field("webhooks.3.name").String()).To(Equal("deny-exec-heritage.admission-policy-engine.deckhouse.io"))
			Expect(vw.Field("webhooks.3.failurePolicy").String()).To(Equal("Fail"))
		})

		It("Guards every request.namespace reference against a cluster-scoped request", func() {
			// A cluster-scoped AdmissionRequest carries no `namespace` key, and reading it raises
			// `no such key: namespace`. A matchCondition that errors rejects the request under
			// failurePolicy: Fail, so every expression touching request.namespace must test for it
			// first. The webhooks here route cluster-scoped objects: `constraints.gatekeeper.sh`
			// is matched with scope `*`.
			for _, resource := range []string{"ValidatingWebhookConfiguration", "MutatingWebhookConfiguration"} {
				wc := f.KubernetesGlobalResource(resource, "d8-admission-policy-engine-config")
				Expect(wc.Exists()).To(BeTrue(), resource)
				for i, webhook := range wc.Field("webhooks").Array() {
					for j, condition := range webhook.Get("matchConditions").Array() {
						expression := condition.Get("expression").String()
						if !strings.Contains(expression, "request.namespace") {
							continue
						}
						Expect(expression).To(ContainSubstring("has(request.namespace)"),
							fmt.Sprintf("%s webhooks.%d.matchConditions.%d", resource, i, j))
					}
				}
			}
		})

		It("Renders MutatingWebhookConfiguration that skips system namespaces", func() {
			mw := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-admission-policy-engine-config")
			Expect(mw.Exists()).To(BeTrue())
			Expect(mw.Field("webhooks.0.namespaceSelector").Exists()).To(BeFalse())
			Expect(mw.Field("webhooks.0.matchConditions.0.expression").String()).To(Equal(
				`!(has(request.namespace) && (request.namespace.startsWith("d8-") || request.namespace.startsWith("kube-")))`))
		})
	})

	Context("Cluster with operator-trivy module enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "operator-trivy"]`)
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.trackedConstraintResources", `[{"apiGroups":[""],"resources":["pods"]}]`)
			f.HelmRender()
		})

		It("Should render without errors when operator-trivy is enabled but denyVulnerableImages is not configured", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})
	})

	Context("Cluster with operator-trivy module and denyVulnerableImages enabled via operatorTrivy values", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "operator-trivy"]`)
			f.ValuesSet("admissionPolicyEngine.internal.bootstrapped", true)
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.trackedConstraintResources", `[{"apiGroups":[""],"resources":["pods"]}]`)
			f.ValuesSet("operatorTrivy.denyVulnerableImages.enabled", true)
			f.HelmRender()
		})

		It("Should render without errors when operatorTrivy.denyVulnerableImages.enabled is true", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})
	})

	Context("Cluster with deckhouse on master node and ratify-provider", func() {
		Context("Enables signature verification", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("admissionPolicyEngine.internal", `{ "bootstrapped": true, "podSecurityStandards": {"enforcementActions": ["deny"]}, "securityPolicies": [{"metadata": {"name": "foo"}, "spec": {"match": {"labelSelector": {}}, "policies": {"verifyImageSignatures": [{"dockerCfg": "zxc=", "reference": "ghcr.io/*", "publicKeys": ["someKey1"]}]}}}], "ratify": {"webhook": {"ca": "ca", "crt": "crt", "key": "key"}, "imageReferences": [{"reference": "ghcr.io/*", "publicKeys": ["someKey1"]}]}, "trackedConstraintResources": [{"apiGroups": [], "resources": ["pod"]}], "webhook": {"ca": "ca", "crt": "crt", "key": "key"}, "trackedMutateResources": []}`)
				f.HelmRender()
			})

			It("Everything must render properly", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("Creates ratify", func() {
				rd := f.KubernetesResource("Deployment", nsName, "ratify")
				Expect(rd.Exists()).To(BeTrue())
				rs := f.KubernetesResource("Service", nsName, "ratify")
				Expect(rs.Exists()).To(BeTrue())
			})

			It("Registry secret stores data from values", func() {
				ratifyRegSecret := f.KubernetesResource("Secret", nsName, "ratify-dockercfg-0")
				Expect(ratifyRegSecret.Exists()).To(BeTrue())
				Expect(ratifyRegSecret.Field(`data.\.dockerconfigjson`).String()).To(Equal("zxc="))
			})
		})
	})
})
