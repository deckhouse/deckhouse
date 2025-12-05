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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
enabledModules: ["vertical-pod-autoscaler", "operator-trivy"]
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

	reportUpdaterValues = `
linkCVEtoBDU: true
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

	operatorDeploy := func(f *Config) object_store.KubeObject {
		deploy := f.KubernetesResource("Deployment", "d8-operator-trivy", "operator")
		Expect(deploy.Exists()).To(BeTrue())
		return deploy
	}

	checkTrivyOperatorEnvs := func(f *Config, name, value string) {
		deploy := operatorDeploy(f)

		operatorContainer := deploy.Field(`spec.template.spec.containers.0.env`).Array()
		for _, env := range operatorContainer {
			if env.Get("name").String() == name {
				Expect(env.Get("value").String()).To(Equal(value))
				return
			}
		}
		Fail(fmt.Sprintf("env %s not found in operator-trivy container", name))
	}

	checkTrivyOperatorDeploy := func(f *Config, tolerations, nodeSelector types.GomegaMatcher) {
		deploy := operatorDeploy(f)
		Expect(deploy.Field("spec.template.spec.tolerations")).To(tolerations)
		Expect(deploy.Field("spec.template.spec.nodeSelector")).To(nodeSelector)
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

		It("Operator trivy has proper tolerations and nodeSelector", func() {
			cmTolerations := `[{"key":"node-role.kubernetes.io/master"},{"key":"node-role.kubernetes.io/control-plane"},{"key":"node.deckhouse.io/etcd-arbiter"},{"key":"dedicated.deckhouse.io","operator":"Exists"},{"key":"dedicated","operator":"Exists"},{"key":"DeletionCandidateOfClusterAutoscaler"},{"key":"ToBeDeletedByClusterAutoscaler"},{"key":"drbd.linbit.com/lost-quorum"},{"key":"drbd.linbit.com/force-io-error"},{"key":"drbd.linbit.com/ignore-fail-over"}]`
			deployTolerations := `[{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-trivy"},{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"},{"key":"drbd.linbit.com/lost-quorum"},{"key":"drbd.linbit.com/force-io-error"},{"key":"drbd.linbit.com/ignore-fail-over"}]`
			nodeSelector := `{"node-role.deckhouse.io/system":""}`
			checkTrivyOperatorCM(f, MatchJSON(cmTolerations), MatchJSON(nodeSelector))
			checkTrivyOperatorDeploy(f, MatchJSON(deployTolerations), MatchJSON(nodeSelector))
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

		It("Operator trivy has proper tolerations and nodeSelector", func() {
			cmTolerations := `[{"key":"node-role.kubernetes.io/master"},{"key":"node-role.kubernetes.io/control-plane"},{"key":"node.deckhouse.io/etcd-arbiter"},{"key":"dedicated.deckhouse.io","operator":"Exists"},{"key":"dedicated","operator":"Exists"},{"key":"DeletionCandidateOfClusterAutoscaler"},{"key":"ToBeDeletedByClusterAutoscaler"},{"key":"drbd.linbit.com/lost-quorum"},{"key":"drbd.linbit.com/force-io-error"},{"key":"drbd.linbit.com/ignore-fail-over"}]`
			deployTolerations := `[{"effect":"NoSchedule","key":"key1","operator":"Equal","value":"value1"},{"key":"drbd.linbit.com/lost-quorum"},{"key":"drbd.linbit.com/force-io-error"},{"key":"drbd.linbit.com/ignore-fail-over"}]`
			nodeSelector := `{"test-label":"test-value"}`
			checkTrivyOperatorCM(f, MatchJSON(cmTolerations), MatchJSON(nodeSelector))
			checkTrivyOperatorDeploy(f, MatchJSON(deployTolerations), MatchJSON(nodeSelector))
		})
	})

	Context("Operator trivy with additional vulnerability report fields set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", `
additionalVulnerabilityReportFields:
- Class
- Target`)

			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy has proper additionalVulnerabilityReportFields set", func() {
			cm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field(`data`).Map()["trivy.additionalVulnerabilityReportFields"].String()).To(Equal("Class,Target"))
		})
	})

	Context("Operator trivy with insecure registry set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", `
insecureRegistries:
- example.com
- test.example.com:8080`)

			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy has proper insecureRegistry.[id] set", func() {
			cm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field(`data`).Map()["trivy.insecureRegistry.0"].String()).To(Equal("example.com"))
			Expect(cm.Field(`data`).Map()["trivy.insecureRegistry.1"].String()).To(Equal("test.example.com:8080"))
			Expect(cm.Field(`data`).Map()["trivy.nonSslRegistry.0"].String()).To(Equal("example.com"))
			Expect(cm.Field(`data`).Map()["trivy.nonSslRegistry.1"].String()).To(Equal("test.example.com:8080"))
		})
	})

	Context("Operator trivy with insecure database registry set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", `
insecureDbRegistry: true`)
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy has proper TRIVY_INSECURE and dbRepositoryInsecure set", func() {
			cm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field(`data`).Map()["trivy.dbRepositoryInsecure"].String()).To(Equal("true"))
			Expect(cm.Field(`data`).Map()["TRIVY_INSECURE"].String()).To(Equal("true"))
		})
	})

	Context("Operator trivy without insecure database registry set", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("Operator trivy has proper TRIVY_INSECURE and dbRepositoryInsecure set", func() {
			cm := f.KubernetesResource("ConfigMap", "d8-operator-trivy", "trivy-operator-trivy-config")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field(`data`).Map()["trivy.dbRepositoryInsecure"].String()).To(Equal("false"))
			Expect(cm.Field(`data`).Map()["TRIVY_INSECURE"].String()).To(Equal("false"))
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

		It("Operator trivy configmap has set several namespaces in target namespaces", func() {
			checkTrivyOperatorEnvs(f, "OPERATOR_TARGET_NAMESPACES", "test,test-1,test-2")
		})
	})

	Context("Operator trivy with linkCVEtoBDU", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("operatorTrivy", reportUpdaterValues)
			f.ValuesSet("operatorTrivy.internal.reportUpdater.webhookCertificate.ca", "test")
			f.ValuesSet("operatorTrivy.internal.reportUpdater.webhookCertificate.crt", "test")
			f.ValuesSet("operatorTrivy.internal.reportUpdater.webhookCertificate.key", "test")
			f.ValuesSet("operatorTrivy.internal.enabledNamespaces", []string{"foo", "bar"})
			f.HelmRender()
		})

		It("Everything must render properly for cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			rwh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "operator-trivy-report-updater")
			Expect(rwh.Exists()).To(BeTrue())
		})
	})
})
