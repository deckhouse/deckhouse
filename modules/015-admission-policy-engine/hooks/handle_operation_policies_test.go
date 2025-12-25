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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
