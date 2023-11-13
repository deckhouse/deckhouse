/*
Copyright 2023 Flant JSC

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

const (
	pssRestrictedProfile = "Restricted"
	pssPrivilegedProfile = "Privileged"
	pssBaselineProfile   = "Baseline"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: detect pss default profile", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"podSecurityStandards": {"enforcementAction": "Deny"},"internal": {"bootstrapped": true, "podSecurityStandards": {"enforcementActions": []}}}}`,
		`{"admissionPolicyEngine": {"podSecurityStandards": {}}}`,
	)

	Context("Empty cluster with podSecurityStandards.defaultProfile preset", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("admissionPolicyEngine.podSecurityStandards.defaultProfile", pssRestrictedProfile)
			f.RunHook()
		})
		It("should have the same default profile", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssRestrictedProfile))
		})
	})

	Context("Cluster without install-data configmap", func() {
		BeforeEach(func() {
			f.RunHook()
		})
		It("should have the default profile set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssPrivilegedProfile))
		})
	})

	Context("Cluster with install-data configmap without version field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(noFieldConfigMap))
			f.RunHook()
		})
		It("should have the default profile set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssPrivilegedProfile))
		})
	})

	Context("Cluster with install-data configmap with incorrect semver in version field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(wrongSemverConfigMap))
			f.RunHook()
		})
		It("should have the default profile set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssPrivilegedProfile))
		})
	})

	Context("Cluster with install-data configmap with v1.54 version field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(v154ConfigMap))
			f.RunHook()
		})
		It("should have the default profile set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssPrivilegedProfile))
		})
	})

	Context("Cluster with install-data configmap with v1.55 version field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(v155ConfigMap))
			f.RunHook()
		})
		It("should have the default profile set to Baseline", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssBaselineProfile))
		})
	})

	Context("Cluster with install-data configmap with v1.56 version field", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(v156ConfigMap))
			f.RunHook()
		})
		It("should have the default profile set to Baseline", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultProfile").String()).To(Equal(pssBaselineProfile))
		})
	})
})

var noFieldConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  not-version: someversion
`

var wrongSemverConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "1.55"
`

var v154ConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "v1.54.1"
`

var v155ConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "v1.55.1"
`

var v156ConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "v1.56.56"
`
