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
	"crypto/sha256"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
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
	speModuleValues = `
internal:
  localPathProvisioners:
  - name: test
    spec:
      path: "/opt/local-path-provisioner"
  - name: trailing
    spec:
      path: "/opt//trailing/"
  - name: quoted
    spec:
      path: "/opt/a: b #c"
  - name: a-very-long-local-path-provisioner-name-that-reaches-the-limit
    spec:
      path: "/long"
`
	speLabel = "security.deckhouse.io/security-policy-exception"
)

func helperPod(cm object_store.KubeObject) gjson.Result {
	podJSON, err := yaml.YAMLToJSON([]byte(cm.Field("data.helperPod\\.yaml").String()))
	Expect(err).ShouldNot(HaveOccurred())
	return gjson.ParseBytes(podJSON)
}

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

	Context("With admission-policy-engine-crd enabled", func() {
		longName := "a-very-long-local-path-provisioner-name-that-reaches-the-limit"
		longSPEName := fmt.Sprintf("helper-%x", sha256.Sum256([]byte(longName)))[:len("helper-")+16]

		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "admission-policy-engine-crd"]`)
			f.ValuesSetFromYaml("localPathProvisioner", speModuleValues)
			f.HelmRender()
		})

		It("Renders the provisioner SecurityPolicyException and labels the Deployments with it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			spe := f.KubernetesResource("SecurityPolicyException", "d8-local-path-provisioner", "local-path-provisioner")
			Expect(spe.Exists()).To(BeTrue())
			Expect(spe.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(spe.Field("spec.network.hostPorts").Exists()).To(BeFalse())
			Expect(spe.Field("spec.securityContext").Exists()).To(BeFalse())
			Expect(spe.Field("spec.volumes").Exists()).To(BeFalse())

			for _, name := range []string{"test", "trailing", longName} {
				deployment := f.KubernetesResource("Deployment", "d8-local-path-provisioner", "local-path-provisioner-"+name)
				Expect(deployment.Exists()).To(BeTrue())
				Expect(deployment.Field("spec.template.metadata.labels").Map()[speLabel].String()).To(Equal("local-path-provisioner"))
			}
		})

		It("Renders a helper SecurityPolicyException per LocalPathProvisioner with the exact host path", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for name, expected := range map[string]struct{ spe, path string }{
				"test":     {"helper-test", "/opt/local-path-provisioner/"},
				"trailing": {"helper-trailing", "/opt/trailing/"},
				"quoted":   {"helper-quoted", "/opt/a: b #c/"},
				longName:   {longSPEName, "/long/"},
			} {
				spe := f.KubernetesResource("SecurityPolicyException", "d8-local-path-provisioner", expected.spe)
				Expect(spe.Exists()).To(BeTrue(), name)
				Expect(spe.Field("spec.securityContext.runAsNonRoot.allowedValue").Bool()).To(BeFalse())
				Expect(spe.Field("spec.securityContext.runAsUser.allowedValues").String()).To(MatchJSON(`[0]`))
				Expect(spe.Field("spec.securityContext.capabilities.allowedValues").String()).To(MatchJSON(`{"add":["DAC_OVERRIDE","FOWNER"],"drop":["ALL"]}`))
				Expect(spe.Field("spec.volumes.types.allowedValues").String()).To(MatchJSON(`["hostPath"]`))
				Expect(spe.Field("spec.volumes.hostPath.allowedValues.#").Int()).To(Equal(int64(1)))
				Expect(spe.Field("spec.volumes.hostPath.allowedValues.0.path").String()).To(Equal(expected.path))
				Expect(spe.Field("spec.volumes.hostPath.allowedValues.0.readOnly").Bool()).To(BeFalse())
				Expect(spe.Field("spec.network").Exists()).To(BeFalse())

				cm := f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-"+name)
				Expect(cm.Exists()).To(BeTrue())
				Expect(helperPod(cm).Get("metadata.labels").Map()[speLabel].String()).To(Equal(expected.spe))
			}
		})

		It("Hardens the helper pod template", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pod := helperPod(f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-test"))
			Expect(pod.Get("spec.securityContext").String()).To(MatchJSON(`{"runAsNonRoot":false,"runAsUser":0,"runAsGroup":0,"seccompProfile":{"type":"RuntimeDefault"}}`))
			Expect(pod.Get("spec.containers.#").Int()).To(Equal(int64(1)))
			Expect(pod.Get("spec.containers.0.securityContext").String()).To(MatchJSON(`{"readOnlyRootFilesystem":true,"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"],"add":["DAC_OVERRIDE","FOWNER"]}}`))
		})
	})

	Context("Without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("localPathProvisioner", speModuleValues)
			f.HelmRender()
		})

		It("Renders no SecurityPolicyExceptions and no exception labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, name := range []string{"local-path-provisioner", "helper-test", "helper-trailing"} {
				Expect(f.KubernetesResource("SecurityPolicyException", "d8-local-path-provisioner", name).Exists()).To(BeFalse())
			}

			deployment := f.KubernetesResource("Deployment", "d8-local-path-provisioner", "local-path-provisioner-test")
			Expect(deployment.Field("spec.template.metadata.labels").Map()).NotTo(HaveKey(speLabel))

			pod := helperPod(f.KubernetesResource("ConfigMap", "d8-local-path-provisioner", "local-path-provisioner-test"))
			Expect(pod.Get("metadata.labels").Exists()).To(BeFalse())
			Expect(pod.Get("spec.containers.0.securityContext.capabilities.drop").String()).To(MatchJSON(`["ALL"]`))
		})
	})
})
