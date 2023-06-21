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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	deprecatedMC = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  annotations:
    meta.helm.sh/release-name: d8-moduleconfig
    meta.helm.sh/release-namespace: d8-moduleconfig
    werf.io/version: v1.2.233
  creationTimestamp: "2022-12-13T11:02:51Z"
  generation: 1
  labels:
    app.kubernetes.io/managed-by: Helm
  name: deckhouse-web
  resourceVersion: "1843862200"
  uid: c377fa26-834c-4cfa-b87d-a876d0872015
spec:
  settings:
    auth:
      password: foobar
      allowedUserGroups:
      - profitbase-flant/access-profitbase-adm
  version: 2
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "2"
`

	documentationMC = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  annotations:
    meta.helm.sh/release-name: d8-moduleconfig
    meta.helm.sh/release-namespace: d8-moduleconfig
    werf.io/version: v1.2.233
  labels:
    app.kubernetes.io/managed-by: Helm
  name: documentation
spec:
  settings:
    auth:
      allowedUserGroups:
      - profitbase-flant/access-profitbase-adm
  version: 1
`
)

func createMCMocks() {
	var oldMC *unstructured.Unstructured
	err := yaml.Unmarshal([]byte(deprecatedMC), &oldMC)
	if err != nil {
		panic(err)
	}

	_, err = dependency.TestDC.MustGetK8sClient().Dynamic().Resource(mcGVR).Create(context.TODO(), oldMC, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Global :: migrate_mc :: deckhouse-web", func() {
	Context("run migration", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createMCMocks()
			f.RunHook()
		})

		It("Hook should create new MC", func() {
			Expect(f).To(ExecuteSuccessfully())
			migrated := getDocumentationMC()
			dd, _ := yaml.Marshal(migrated)
			Expect(dd).To(MatchYAML(documentationMC))
		})
	})
})

func getDocumentationMC() *unstructured.Unstructured {
	un, err := dependency.TestDC.MustGetK8sClient().Dynamic().Resource(mcGVR).Get(context.TODO(), "documentation", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	return un
}
