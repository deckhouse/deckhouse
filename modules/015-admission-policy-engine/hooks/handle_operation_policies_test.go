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

package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"

	v1alpha1 "github.com/deckhouse/deckhouse/modules/015-admission-policy-engine/hooks/internal/apis"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: handle operation policies", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"bootstrapped": true} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	f.RegisterCRD("templates.gatekeeper.sh", "v1", "ConstraintTemplate", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "OperationPolicy", false)

	Context("Preserve explicit empty arrays in Values for selected fields", func() {
		Context("Case A: allowedRepos is omitted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testOperationPolicyAllowedReposOmitted))
				f.RunHook()
			})
			It("should not include allowedRepos key in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				ops := f.ValuesGet("admissionPolicyEngine.internal.operationPolicies").Array()
				Expect(ops).To(HaveLen(1))
				Expect(ops[0].Get("spec.policies.allowedRepos").Exists()).To(BeFalse())
			})
		})

		Context("Case B: allowedRepos is explicitly set to []", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testOperationPolicyAllowedReposEmpty))
				f.RunHook()
			})
			It("should include allowedRepos key with empty array in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				ops := f.ValuesGet("admissionPolicyEngine.internal.operationPolicies").Array()
				Expect(ops).To(HaveLen(1))
				Expect(ops[0].Get("spec.policies.allowedRepos").Exists()).To(BeTrue())
				Expect(ops[0].Get("spec.policies.allowedRepos").Array()).To(HaveLen(0))
			})
		})

		Context("Case C: allowedRepos is set with one item", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testOperationPolicy))
				f.RunHook()
			})
			It("should include allowedRepos key with non-empty array in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				ops := f.ValuesGet("admissionPolicyEngine.internal.operationPolicies").Array()
				Expect(ops).To(HaveLen(1))
				Expect(ops[0].Get("spec.policies.allowedRepos").Exists()).To(BeTrue())
				Expect(ops[0].Get("spec.policies.allowedRepos").Array()).To(HaveLen(1))
			})
		})

		Context("Nested: requiredResources.limits is explicitly set to []", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testOperationPolicyRequiredResourcesLimitsEmpty))
				f.RunHook()
			})
			It("should include requiredResources.limits key with empty array in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				ops := f.ValuesGet("admissionPolicyEngine.internal.operationPolicies").Array()
				Expect(ops).To(HaveLen(1))
				Expect(ops[0].Get("spec.policies.requiredResources.limits").Exists()).To(BeTrue())
				Expect(ops[0].Get("spec.policies.requiredResources.limits").Array()).To(HaveLen(0))
			})
		})
	})

	Context("Pointer slice semantics: omit vs [] vs non-empty (operation policies)", func() {
		type sliceCase struct {
			name         string
			path         string
			omitSnippet  string
			emptySnippet string
			nonEmpty     string
		}

		operationPolicyYAML := func(policiesSnippet string) string {
			return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      matchNames: ["default"]
  policies:
%s
`, policiesSnippet)
		}

		runAndGet := func(yaml string) gjson.Result {
			f.BindingContexts.Set(f.KubeStateSet(yaml))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			arr := f.ValuesGet("admissionPolicyEngine.internal.operationPolicies").Array()
			Expect(arr).To(HaveLen(1))
			return arr[0]
		}

		cases := []sliceCase{
			{
				name:         "allowedRepos",
				path:         "spec.policies.allowedRepos",
				omitSnippet:  "",
				emptySnippet: "    allowedRepos: []",
				nonEmpty: "    allowedRepos:\n" +
					"      - foo",
			},
			{
				name:         "requiredResources.limits",
				path:         "spec.policies.requiredResources.limits",
				omitSnippet:  "",
				emptySnippet: "    requiredResources:\n      limits: []",
				nonEmpty: "    requiredResources:\n" +
					"      limits: [\"memory\"]",
			},
			{
				name:         "requiredResources.requests",
				path:         "spec.policies.requiredResources.requests",
				omitSnippet:  "",
				emptySnippet: "    requiredResources:\n      requests: []",
				nonEmpty: "    requiredResources:\n" +
					"      requests: [\"cpu\"]",
			},
			{
				name:         "disallowedImageTags",
				path:         "spec.policies.disallowedImageTags",
				omitSnippet:  "",
				emptySnippet: "    disallowedImageTags: []",
				nonEmpty: "    disallowedImageTags:\n" +
					"      - latest",
			},
			{
				name:         "requiredProbes",
				path:         "spec.policies.requiredProbes",
				omitSnippet:  "",
				emptySnippet: "    requiredProbes: []",
				nonEmpty: "    requiredProbes:\n" +
					"      - livenessProbe",
			},
			{
				name:         "priorityClassNames",
				path:         "spec.policies.priorityClassNames",
				omitSnippet:  "",
				emptySnippet: "    priorityClassNames: []",
				nonEmpty: "    priorityClassNames:\n" +
					"      - production-high",
			},
			{
				name:         "ingressClassNames",
				path:         "spec.policies.ingressClassNames",
				omitSnippet:  "",
				emptySnippet: "    ingressClassNames: []",
				nonEmpty: "    ingressClassNames:\n" +
					"      - nginx",
			},
			{
				name:         "storageClassNames",
				path:         "spec.policies.storageClassNames",
				omitSnippet:  "",
				emptySnippet: "    storageClassNames: []",
				nonEmpty: "    storageClassNames:\n" +
					"      - standard",
			},
			{
				name:         "disallowedTolerations",
				path:         "spec.policies.disallowedTolerations",
				omitSnippet:  "",
				emptySnippet: "    disallowedTolerations: []",
				nonEmpty: "    disallowedTolerations:\n" +
					"      - key: node-role.kubernetes.io/master\n" +
					"        operator: Exists",
			},
		}

		It("should preserve slice tri-state semantics in Values", func() {
			for _, tc := range cases {
				By("omit: " + tc.name)
				o := runAndGet(operationPolicyYAML(tc.omitSnippet))
				Expect(o.Get(tc.path).Exists()).To(BeFalse())

				By("empty: " + tc.name)
				e := runAndGet(operationPolicyYAML(tc.emptySnippet))
				Expect(e.Get(tc.path).Exists()).To(BeTrue())
				Expect(e.Get(tc.path).Array()).To(HaveLen(0))

				By("non-empty: " + tc.name)
				n := runAndGet(operationPolicyYAML(tc.nonEmpty))
				Expect(n.Get(tc.path).Exists()).To(BeTrue())
				Expect(n.Get(tc.path).Array()).ToNot(BeEmpty())
			}
		})

		It("should omit replicaLimits when not specified and include when set", func() {
			o := runAndGet(operationPolicyYAML(""))
			Expect(o.Get("spec.policies.replicaLimits").Exists()).To(BeFalse())

			n := runAndGet(operationPolicyYAML("    replicaLimits:\n      minReplicas: 1\n      maxReplicas: 3"))
			Expect(n.Get("spec.policies.replicaLimits").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.replicaLimits.minReplicas").Int()).To(Equal(int64(1)))
			Expect(n.Get("spec.policies.replicaLimits.maxReplicas").Int()).To(Equal(int64(3)))
		})

		It("should omit requiredResources completely when not specified", func() {
			o := runAndGet(operationPolicyYAML(""))
			Expect(o.Get("spec.policies.requiredResources").Exists()).To(BeFalse())
		})

		It("should keep requiredResources when both limits and requests are empty slices", func() {
			e := runAndGet(operationPolicyYAML("    requiredResources:\n      limits: []\n      requests: []"))
			Expect(e.Get("spec.policies.requiredResources").Exists()).To(BeTrue())
			Expect(e.Get("spec.policies.requiredResources.limits").Array()).To(HaveLen(0))
			Expect(e.Get("spec.policies.requiredResources.requests").Array()).To(HaveLen(0))
		})

		It("should keep requiredResources when both limits and requests are non-empty", func() {
			n := runAndGet(operationPolicyYAML("    requiredResources:\n      limits: [\"memory\"]\n      requests: [\"cpu\"]"))
			Expect(n.Get("spec.policies.requiredResources").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.requiredResources.limits").Array()).ToNot(BeEmpty())
			Expect(n.Get("spec.policies.requiredResources.requests").Array()).ToNot(BeEmpty())
		})

		It("should include maxRevisionHistoryLimit only when set", func() {
			o := runAndGet(operationPolicyYAML(""))
			Expect(o.Get("spec.policies.maxRevisionHistoryLimit").Exists()).To(BeFalse())

			n := runAndGet(operationPolicyYAML("    maxRevisionHistoryLimit: 5"))
			Expect(n.Get("spec.policies.maxRevisionHistoryLimit").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.maxRevisionHistoryLimit").Int()).To(Equal(int64(5)))
		})

		It("should include imagePullPolicy only when set", func() {
			o := runAndGet(operationPolicyYAML(""))
			Expect(o.Get("spec.policies.imagePullPolicy").Exists()).To(BeFalse())

			n := runAndGet(operationPolicyYAML("    imagePullPolicy: Always"))
			Expect(n.Get("spec.policies.imagePullPolicy").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.imagePullPolicy").String()).To(Equal("Always"))
		})

		It("should keep booleans when set", func() {
			n := runAndGet(operationPolicyYAML("    checkHostNetworkDNSPolicy: true\n    checkContainerDuplicates: true"))
			Expect(n.Get("spec.policies.checkHostNetworkDNSPolicy").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.checkHostNetworkDNSPolicy").Bool()).To(BeTrue())
			Expect(n.Get("spec.policies.checkContainerDuplicates").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.checkContainerDuplicates").Bool()).To(BeTrue())
		})

		It("should keep replicaLimits when only one of min/max is set", func() {
			minOnly := runAndGet(operationPolicyYAML("    replicaLimits:\n      minReplicas: 2"))
			Expect(minOnly.Get("spec.policies.replicaLimits").Exists()).To(BeTrue())
			Expect(minOnly.Get("spec.policies.replicaLimits.minReplicas").Int()).To(Equal(int64(2)))
			Expect(minOnly.Get("spec.policies.replicaLimits.maxReplicas").Exists()).To(BeFalse())

			maxOnly := runAndGet(operationPolicyYAML("    replicaLimits:\n      maxReplicas: 4"))
			Expect(maxOnly.Get("spec.policies.replicaLimits").Exists()).To(BeTrue())
			Expect(maxOnly.Get("spec.policies.replicaLimits.maxReplicas").Int()).To(Equal(int64(4)))
			Expect(maxOnly.Get("spec.policies.replicaLimits.minReplicas").Exists()).To(BeFalse())
		})

		It("should not add unrelated keys when only allowedRepos is set", func() {
			only := runAndGet(operationPolicyYAML("    allowedRepos:\n      - foo.registry"))
			Expect(only.Get("spec.policies.allowedRepos").Exists()).To(BeTrue())
			Expect(only.Get("spec.policies.allowedRepos").Array()).ToNot(BeEmpty())
			Expect(only.Get("spec.policies.requiredResources").Exists()).To(BeFalse())
			Expect(only.Get("spec.policies.replicaLimits").Exists()).To(BeFalse())
			Expect(only.Get("spec.policies.disallowedImageTags").Exists()).To(BeFalse())
		})

		It("should preserve omit vs non-empty for requiredLabels/requiredAnnotations (CRD may forbid empty arrays)", func() {
			// requiredLabels.labels
			o := runAndGet(operationPolicyYAML(""))
			Expect(o.Get("spec.policies.requiredLabels.labels").Exists()).To(BeFalse())

			n := runAndGet(operationPolicyYAML("    requiredLabels:\n      labels:\n      - key: product-id\n        allowedRegex: ^P\\d{4}$\n      watchKinds: [\"/Namespace\"]"))
			Expect(n.Get("spec.policies.requiredLabels.labels").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.requiredLabels.labels").Array()).ToNot(BeEmpty())
			Expect(n.Get("spec.policies.requiredLabels.watchKinds").Exists()).To(BeTrue())
			Expect(n.Get("spec.policies.requiredLabels.watchKinds").Array()).ToNot(BeEmpty())

			// requiredAnnotations.annotations
			o2 := runAndGet(operationPolicyYAML(""))
			Expect(o2.Get("spec.policies.requiredAnnotations.annotations").Exists()).To(BeFalse())

			n2 := runAndGet(operationPolicyYAML("    requiredAnnotations:\n      annotations:\n      - key: foobar\n        allowedRegex: ^P\\d{4}$\n      watchKinds: [\"/Namespace\"]"))
			Expect(n2.Get("spec.policies.requiredAnnotations.annotations").Exists()).To(BeTrue())
			Expect(n2.Get("spec.policies.requiredAnnotations.annotations").Array()).ToNot(BeEmpty())
			Expect(n2.Get("spec.policies.requiredAnnotations.watchKinds").Exists()).To(BeTrue())
			Expect(n2.Get("spec.policies.requiredAnnotations.watchKinds").Array()).ToNot(BeEmpty())
		})
	})
})

func TestMarshalOperationPolicy(t *testing.T) {
	var tmp map[string]any
	err := yaml.Unmarshal([]byte(testOperationPolicy), &tmp)
	if err != nil {
		t.Error(err)
	}

	jsonSpec, err := json.Marshal(tmp["spec"])
	if err != nil {
		t.Error(err)
	}

	var spec v1alpha1.OperationPolicySpec
	dec := json.NewDecoder(bytes.NewBuffer(jsonSpec))
	dec.DisallowUnknownFields()
	err = dec.Decode(&spec)
	if err != nil {
		t.Error(err)
	}
}

var testOperationPolicy = `
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: foo
spec:
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
      - allowedRegex: ^P\d{4}$
        key: product-id
      watchKinds:
      - /Namespace
    requiredAnnotations:
      annotations:
      - allowedRegex: ^P\d{4}$
        key: foobar
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
      maxReplicas: 3
  match:
    namespaceSelector:
      matchNames:
        - default
`

var testOperationPolicyAllowedReposOmitted = `
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: foo
spec:
  policies:
    requiredProbes:
      - livenessProbe
  match:
    namespaceSelector:
      matchNames:
        - default
`

var testOperationPolicyAllowedReposEmpty = `
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: foo
spec:
  policies:
    allowedRepos: []
    requiredProbes:
      - livenessProbe
  match:
    namespaceSelector:
      matchNames:
        - default
`

var testOperationPolicyRequiredResourcesLimitsEmpty = `
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: foo
spec:
  policies:
    requiredResources:
      limits: []
  match:
    namespaceSelector:
      matchNames:
        - default
`
