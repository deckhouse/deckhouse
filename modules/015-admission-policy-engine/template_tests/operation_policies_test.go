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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	testPolicyName = "genpolicy"
)

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: operation policies", func() {
	f := SetupHelmConfig(`
global:
  discovery:
    kubernetesVersion: "1.31"
admissionPolicyEngine:
  podSecurityStandards: {}
  internal:
    bootstrapped: true
    ratify:
      webhook:
        key: YjY0ZW5jX3N0cmluZwo=
        crt: YjY0ZW5jX3N0cmluZwo=
        ca: YjY0ZW5jX3N0cmluZwo=
    podSecurityStandards:
      enforcementActions:
        - deny
    operationPolicies:
      - metadata:
          name: genpolicy
        spec:
          enforcementAction: Warn
          policies:
            allowedRepos:
              - foo
            requiredResources:
              limits:
                - memory
              requests:
                - cpu
                - memory
            disallowedImageTags:
              - latest
            requiredLabels:
              labels:
                - key: foo
                - key: bar
                  allowedRegex: "^[a-zA-Z]+.agilebank.demo$"
              watchKinds:
                - /Pod
                - networking.k8s.io/Ingress
            requiredAnnotations:
              annotations:
                - key: foo
                - key: bar
                  allowedRegex: "^[a-zA-Z]+.myapp.demo$"
              watchKinds:
                - /Namespace
            requiredProbes:
              - livenessProbe
              - readinessProbe
            maxRevisionHistoryLimit: 3
            imagePullPolicy: Always
            priorityClassNames:
              - foo
              - bar
            ingressClassNames:
              - ing1
              - ing2
            storageClassNames:
              - st1
              - st2
            checkHostNetworkDNSPolicy: true
            checkContainerDuplicates: true
            replicaLimits:
              minReplicas: 1
              maxReplicas: 10
            disallowedTolerations:
              - key: node-role.kubernetes.io/master
                operator: Exists
              - key: node-role.kubernetes.io/control-plane
                operator: Exists
          match:
            namespaceSelector:
              matchNames:
                - default
              excludeNames:
                - kube-system
              labelSelector:
                matchLabels:
                  operation-policy.deckhouse.io/enabled: "true"
            labelSelector:
              matchLabels:
                operation-policy.deckhouse.io/enabled: "true"
    trackedConstraintResources:
      - apiGroups:
          - ""
        resources:
          - pods
          - nodes
          - namespaces
      - apiGroups:
          - extensions
          - networking.k8s.io
        resources:
          - ingresses
    trackedMutateResources:
      - apiGroups:
          - ""
        resources:
          - pods
      - apiGroups:
          - extensions
          - networking.k8s.io
        resources:
          - ingresses
    webhook:
      ca: YjY0ZW5jX3N0cmluZwo=
      crt: YjY0ZW5jX3N0cmluZwo=
      key: YjY0ZW5jX3N0cmluZwo=
`)

	Context("Cluster with operation policies", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("D8AllowedRepos", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredResources", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DisallowedTags", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredProbes", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RevisionHistoryLimit", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ImagePullPolicy", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8PriorityClass", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8IngressClass", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8StorageClass", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DNSPolicy", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredLabels", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredAnnotations", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ContainerDuplicates", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ReplicaLimits", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DisallowedTolerations", testPolicyName).Exists()).To(BeTrue())
		})

		It("All operation policy constraints must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Dynamically extract constraint names from template files
			operationConstraints := getOperationConstraintNames()
			Expect(operationConstraints).NotTo(BeEmpty(), "No operation constraints found in templates")

			for _, constraintKind := range operationConstraints {
				constraint := f.KubernetesGlobalResource(constraintKind, testPolicyName)
				if constraint.Exists() {
					// Get the resource as a map to validate YAML structure
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, constraintKind)
				}
			}
		})

		It("Operation policy constraints must use values for enforcementAction, match and parameters", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			expectedSelector := constraintSelectorExpectation{
				namespaces:         mustParseYaml("- default"),
				excludedNamespaces: mustParseYaml("- kube-system"),
				namespaceSelector:  mustParseYaml("matchLabels:\n  operation-policy.deckhouse.io/enabled: \"true\""),
				labelSelector:      mustParseYaml("matchLabels:\n  operation-policy.deckhouse.io/enabled: \"true\""),
			}
			expectedAction := "warn"

			expectedParameters := map[string]interface{}{
				"D8AllowedRepos":          mustParseYaml("repos:\n  - foo"),
				"D8RequiredResources":     mustParseYaml("limits:\n  - memory\nrequests:\n  - cpu\n  - memory"),
				"D8DisallowedTags":        mustParseYaml("tags:\n  - latest"),
				"D8RequiredLabels":        mustParseYaml("labels:\n  - key: foo\n  - key: bar\n    allowedRegex: \"^[a-zA-Z]+.agilebank.demo$\""),
				"D8RequiredAnnotations":   mustParseYaml("annotations:\n  - key: foo\n  - key: bar\n    allowedRegex: \"^[a-zA-Z]+.myapp.demo$\""),
				"D8RequiredProbes":        mustParseYaml("probes:\n  - livenessProbe\n  - readinessProbe"),
				"D8RevisionHistoryLimit":  mustParseYaml("limit: 3"),
				"D8ImagePullPolicy":       mustParseYaml("policy: \"Always\""),
				"D8PriorityClass":         mustParseYaml("priorityClassNames:\n  - foo\n  - bar"),
				"D8IngressClass":          mustParseYaml("ingressClassNames:\n  - ing1\n  - ing2"),
				"D8StorageClass":          mustParseYaml("storageClassNames:\n  - st1\n  - st2"),
				"D8ReplicaLimits":         mustParseYaml("ranges:\n  - minReplicas: 1\n    maxReplicas: 10"),
				"D8DisallowedTolerations": mustParseYaml("tolerations:\n  - key: node-role.kubernetes.io/master\n    operator: Exists\n  - key: node-role.kubernetes.io/control-plane\n    operator: Exists"),
			}

			constraintsWithoutParameters := []string{
				"D8DNSPolicy",
				"D8ContainerDuplicates",
			}

			for constraintKind, expected := range expectedParameters {
				constraint := f.KubernetesGlobalResource(constraintKind, testPolicyName)
				Expect(constraint.Exists()).To(BeTrue())
				spec := getConstraintSpecMap(constraint)
				expectConstraintAction(spec, expectedAction)
				expectConstraintSelector(spec, expectedSelector)
				expectConstraintParameters(spec, expected)
			}

			for _, constraintKind := range constraintsWithoutParameters {
				constraint := f.KubernetesGlobalResource(constraintKind, testPolicyName)
				Expect(constraint.Exists()).To(BeTrue())
				spec := getConstraintSpecMap(constraint)
				expectConstraintAction(spec, expectedAction)
				expectConstraintSelector(spec, expectedSelector)
				expectConstraintParameters(spec, nil)
			}
		})
	})
})
