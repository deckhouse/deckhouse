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
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const testRoot = "testdata/required_constraint_templates"

var _ = Describe("Modules :: admission-policy-engine :: hooks :: bootstrap_handler", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"bootstrapped": false} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	f.RegisterCRD("templates.gatekeeper.sh", "v1", "ConstraintTemplate", true)
	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/empty/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())
		})
	})

	Context("Deployment not ready", func() {
		BeforeEach(func() {
			f.KubeStateSet(notReadyDeployment)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())
		})
	})

	Context("Deployment is ready, no templates required", func() {
		BeforeEach(func() {
			f.KubeStateSet(readyDeployment)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/empty/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeTrue())
		})
	})

	Context("Deployment is ready, some templates are missing", func() {
		BeforeEach(func() {
			f.KubeStateSet(readyDeployment + constraintTemplate1)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeFalse())
		})
	})

	Context("Deployment is ready, required templates are in place", func() {
		BeforeEach(func() {
			f.KubeStateSet(readyDeployment + constraintTemplate1 + constraintTemplate2)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			err := setTestChartPath(fmt.Sprintf("%s/valid/templates", testRoot))
			Expect(err).To(BeNil())
			f.RunHook()
		})
		It("should keep bootstrapped flag as true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.bootstrapped").Bool()).To(BeTrue())
		})
	})
})

func setTestChartPath(path string) error {
	return os.Setenv("D8_TEST_CHART_PATH", path)
}

var readyDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: gatekeeper
    app.kubernetes.io/managed-by: Helm
    control-plane: controller-manager
    heritage: deckhouse
    module: admission-policy-engine
  name: gatekeeper-controller-manager
  namespace: d8-admission-policy-engine
spec:
  progressDeadlineSeconds: 600
  replicas: 1
status:
  availableReplicas: 1
  observedGeneration: 1
  readyReplicas: 1
  replicas: 1
  updatedReplicas: 1
`

var notReadyDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: gatekeeper
    app.kubernetes.io/managed-by: Helm
    control-plane: controller-manager
    heritage: deckhouse
    module: admission-policy-engine
  name: gatekeeper-controller-manager
  namespace: d8-admission-policy-engine
spec:
  progressDeadlineSeconds: 600
  replicas: 1
status:
  availableReplicas: 0
  readyReplicas: 0
  replicas: 1
  updatedReplicas: 1
`

var constraintTemplate1 = `
---
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8priorityclass
  labels:
    heritage: deckhouse
    module: admission-policy-engine
    security.deckhouse.io: operation-policy
  annotations:
    metadata.gatekeeper.sh/title: "Required Priority Class"
    metadata.gatekeeper.sh/version: 1.0.0
    description: "Required Priority Class"
`

var constraintTemplate2 = `
---
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8readonlyrootfilesystem
  labels:
    heritage: deckhouse
    module: admission-policy-engine
    security.deckhouse.io: security-policy
  annotations:
    metadata.gatekeeper.sh/title: "Read Only Root Filesystem"
    metadata.gatekeeper.sh/version: 1.0.0
`
