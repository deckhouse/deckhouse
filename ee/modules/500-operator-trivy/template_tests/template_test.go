/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "operator-trivy"]
modulesImages:
  registry:
    base: registry.deckhouse.io/deckhouse/fe
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterDomain: cluster.local
`

	scanJobsValues = `
tolerations:
  - key: "key1"
    operator: "Equal"
    value: "value1"
    effect: "NoSchedule"
nodeSelector:
  test-label: test-value
`
)

var _ = Describe("Module :: operator-trivy :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	checkTrivyOperatorCM := func(f *Config, tolerations, nodeSelector types.GomegaMatcher) {
		cm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator")
		Expect(cm.Exists()).To(BeTrue())

		cmdData := cm.Field(`data`).Map()

		Expect(cmdData["scanJob.tolerations"].String()).To(tolerations)
		Expect(cmdData["scanJob.nodeSelector"].String()).To(nodeSelector)

	}

	checkTrivyOperatorEnvs := func(f *Config, name, value string) {
		deploy := f.KubernetesResource("Deployment", "d8-operator-trivy", "operator")
		Expect(deploy.Exists()).To(BeTrue())

		operatorContainer := deploy.Field(`spec.template.spec.containers.0.env`).Array()

		for _, env := range operatorContainer {
			if env.Get("name").String() == name {
				Expect(env.Get("value").String()).To(Equal(value))
				return
			}
		}
		Fail(fmt.Sprintf("env %s not found in operator-trivy container", name))
	}

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	Context("Default", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has proper tolerations and nodeSelector", func() {
			checkTrivyOperatorCM(f, Equal(""), Equal(""))
		})
	})

	Context("Operator trivy with custom tolerations and nodeSelector", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", scanJobsValues)
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has proper tolerations and nodeSelector", func() {
			tolerations := `[{"effect":"NoSchedule","key":"key1","operator":"Equal","value":"value1"}]`
			nodeSelector := `{"test-label":"test-value"}`
			checkTrivyOperatorCM(f, MatchJSON(tolerations), MatchJSON(nodeSelector))
		})
	})

	Context("Operator trivy with no value in enabledNamespaces", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has set default namespace to target namespaces", func() {
			checkTrivyOperatorEnvs(f, "OPERATOR_TARGET_NAMESPACES", "default")
		})
	})

	Context("Operator trivy with zero len enabledNamespaces", func() {
		BeforeEach(func() {
			f.ValuesSet("operatorTrivy.internal.enabledNamespaces", []string{})
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has set default namespace to target namespaces", func() {
			checkTrivyOperatorEnvs(f, "OPERATOR_TARGET_NAMESPACES", "default")
		})
	})

	Context("Operator trivy with enabledNamespaces", func() {
		BeforeEach(func() {
			f.ValuesSet("operatorTrivy.internal.enabledNamespaces", []string{"test", "test-1", "test-2"})
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy configmap has set default namespace to target namespaces", func() {
			checkTrivyOperatorEnvs(f, "OPERATOR_TARGET_NAMESPACES", "test,test-1,test-2")
		})
	})
})
