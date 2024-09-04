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

package template_tests

import (
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
	globalValues = `
  enabledModules: ["vertical-pod-autoscaler"]
  modules:
    placement: {}
  discovery:
    kubernetesVersion: 1.20.5
    d8SpecificNodeCountByRole:
      worker: 3
      master: 3
`
	moduleValues = `
internal:
  localPathProvisioners:
  - name: test
    spec:
      nodeGroups:
      - master
      - worker
      path: "/opt/local-path-provisioner"
  - name: test2
    spec:
      nodeGroups:
      - worker
      path: "/local"
  - name: test3
    spec:
      path: "/local3"
`
)

var _ = Describe("Module :: local-path-provisioner :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Two local path provisioner instances", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("localPathProvisioner", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-local-path-provisioner")
			registrySecret := f.KubernetesResource("Secret", "d8-local-path-provisioner", "deckhouse-registry")
			lppServiceAccount := f.KubernetesResource("ServiceAccount", "d8-local-path-provisioner", "local-path-provisioner")
			lppClusterRole := f.KubernetesGlobalResource("ClusterRole", "d8:local-path-provisioner")
			lppClusterRoleBinding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:local-path-provisioner")

			lppConfigMapTest := f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-test")
			lppSCTest := f.KubernetesGlobalResource("StorageClass", "test")
			lppDeploymentTest := f.KubernetesResource("Deployment", "d8-local-path-provisioner", "local-path-provisioner-test")
			lppVPATest := f.KubernetesResource("VerticalPodAutoscaler", "d8-local-path-provisioner", "local-path-provisioner-test")
			lppPDBTest := f.KubernetesResource("PodDisruptionBudget", "d8-local-path-provisioner", "local-path-provisioner-test")

			lppConfigMapTest2 := f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-test2")
			lppSCTest2 := f.KubernetesGlobalResource("StorageClass", "test2")
			lppDeploymentTest2 := f.KubernetesResource("Deployment", "d8-local-path-provisioner", "local-path-provisioner-test2")
			lppVPATest2 := f.KubernetesResource("VerticalPodAutoscaler", "d8-local-path-provisioner", "local-path-provisioner-test2")
			lppPDBTest2 := f.KubernetesResource("PodDisruptionBudget", "d8-local-path-provisioner", "local-path-provisioner-test2")

			lppConfigMapTest3 := f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-test3")
			lppSCTest3 := f.KubernetesGlobalResource("StorageClass", "test3")
			lppDeploymentTest3 := f.KubernetesResource("Deployment", "d8-local-path-provisioner", "local-path-provisioner-test3")
			lppVPATest3 := f.KubernetesResource("VerticalPodAutoscaler", "d8-local-path-provisioner", "local-path-provisioner-test3")
			lppPDBTest3 := f.KubernetesResource("PodDisruptionBudget", "d8-local-path-provisioner", "local-path-provisioner-test3")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(lppServiceAccount.Exists()).To(BeTrue())
			Expect(lppClusterRole.Exists()).To(BeTrue())
			Expect(lppClusterRoleBinding.Exists()).To(BeTrue())

			Expect(lppConfigMapTest.Exists()).To(BeTrue())
			Expect(lppSCTest.Exists()).To(BeTrue())
			Expect(lppDeploymentTest.Exists()).To(BeTrue())
			Expect(lppVPATest.Exists()).To(BeTrue())
			Expect(lppPDBTest.Exists()).To(BeTrue())
			Expect(lppConfigMapTest.Field("data.config\\.json").String()).To(MatchJSON(`
{
        "nodePathMap":[
        {
                "node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
                "paths":["/opt/local-path-provisioner"]
        }
        ],
		"setupCommand": "/manager",
        "teardownCommand": "/manager"
}`))
			Expect(lppSCTest.Field("allowedTopologies").String()).To(MatchJSON(`
[
  {
	"matchLabelExpressions": [
	  {
		"key": "node.deckhouse.io/group",
		"values": [
		  "master",
          "worker"
		]
	  }
	]
  }
]
`))

			Expect(lppConfigMapTest2.Exists()).To(BeTrue())
			Expect(lppSCTest2.Exists()).To(BeTrue())
			Expect(lppDeploymentTest2.Exists()).To(BeTrue())
			Expect(lppVPATest2.Exists()).To(BeTrue())
			Expect(lppPDBTest2.Exists()).To(BeTrue())
			Expect(lppConfigMapTest2.Field("data.config\\.json").String()).To(MatchJSON(`
{
		"nodePathMap":[
		{
			"node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
			"paths":["/local"]
		}
		],
		"setupCommand": "/manager",
        "teardownCommand": "/manager"
}`))
			Expect(lppSCTest2.Field("allowedTopologies").String()).To(MatchJSON(`
[
  {
	"matchLabelExpressions": [
	  {
		"key": "node.deckhouse.io/group",
		"values": [
		  "worker"
		]
	  }
	]
  }
]
`))

			Expect(lppConfigMapTest3.Exists()).To(BeTrue())
			Expect(lppSCTest3.Exists()).To(BeTrue())
			Expect(lppDeploymentTest3.Exists()).To(BeTrue())
			Expect(lppVPATest3.Exists()).To(BeTrue())
			Expect(lppPDBTest3.Exists()).To(BeTrue())
			Expect(lppConfigMapTest3.Field("data.config\\.json").String()).To(MatchJSON(`
{
		"nodePathMap":[
		{
			"node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
			"paths":["/local3"]
		}
		],
		"setupCommand": "/manager",
        "teardownCommand": "/manager"
}`))
			Expect(lppSCTest3.Field("allowedTopologies").Exists()).To(BeFalse())
		})
	})
})
