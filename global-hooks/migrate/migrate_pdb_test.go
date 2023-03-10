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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	shouldMigrate = `
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  annotations:
    helm.sh/hook: post-upgrade, post-install
    helm.sh/hook-delete-policy: before-hook-creation
  labels:
    app: webhook-handler
    heritage: deckhouse
    module: deckhouse
  name: webhook-handler
  namespace: d8-system
spec:
  minAvailable: 0
  selector:
    matchLabels:
      app: webhook-handler
`

	shouldntMigrate = `
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  annotations:
    meta.helm.sh/release-name: cni-cilium
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: operator
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cni-cilium
  name: operator
  namespace: d8-cni-cilium
spec:
  minAvailable: 0
  selector:
    matchLabels:
      app: operator
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`
)

func createMocks() {
	var pdb policyv1.PodDisruptionBudget
	err := yaml.Unmarshal([]byte(shouldMigrate), &pdb)
	if err != nil {
		panic(err)
	}
	_, err = dependency.TestDC.MustGetK8sClient().PolicyV1().PodDisruptionBudgets(pdb.Namespace).Create(context.TODO(), &pdb, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal([]byte(shouldntMigrate), &pdb)
	if err != nil {
		panic(err)
	}
	_, err = dependency.TestDC.MustGetK8sClient().PolicyV1().PodDisruptionBudgets(pdb.Namespace).Create(context.TODO(), &pdb, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Global :: migrate_pdbs ::", func() {
	Context("run migration", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMocks()
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			migrated := getPDB("d8-system", "webhook-handler")
			Expect(migrated.Labels["app.kubernetes.io/managed-by"]).To(Equal("Helm"))

			Expect(migrated.Annotations["meta.helm.sh/release-name"]).To(Equal("deckhouse"))
			Expect(migrated.Annotations["meta.helm.sh/release-namespace"]).To(Equal("d8-system"))

			Expect(migrated.Annotations["helm.sh/hook"]).To(BeEmpty())
			Expect(migrated.Annotations["helm.sh/hook-delete-policy"]).To(BeEmpty())

			notMigrated := getPDB("d8-cni-cilium", "operator")
			dd, _ := yaml.Marshal(notMigrated)
			Expect(dd).To(MatchYAML(shouldntMigrate))
		})
	})
})

func getPDB(ns, name string) *policyv1.PodDisruptionBudget {
	pdb, err := dependency.TestDC.MustGetK8sClient().PolicyV1().PodDisruptionBudgets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	return pdb
}
