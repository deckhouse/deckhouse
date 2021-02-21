package template_tests

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// Note double percent sign in "publicDomainTemplate" field to preserve "%s" placeholder.
const globalValuesFmt = `
enabledModules: [%q]
modulesImages:
  registry: registry.flant.com
  registryDockercfg: cfg
  tags:
    common:
      kubeCaAuthProxy: tagstring
      kubeRbacProxy: tagstring
    upmeter:
      smokeMini: tagstring
      status: tagstring
      upmeter: tagstring
      webui: tagstring
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  kubernetesVersion: 1.16.15
`
const upmeterValuesFmt = `
smokeMiniDisabled: %t
`

var _ = Describe("Module :: upmeter :: helm template :: disabled probes", func() {

	Context("upmeter-agent UPMETER_DISABLED_PROBES", func() {
		f := SetupHelmConfig(``)
		renderer := newDisabledProbesRenderer(f)

		It("includes synthetic group when smokeMiniDisabled=true", func() {
			value := renderer.WithMiniDisabled(true)
			Expect(value).To(ContainElement("synthetic/"))
		})

		It("includes synthetic group when smokeMiniDisabled=false", func() {
			value := renderer.WithMiniDisabled(false)
			Expect(value).NotTo(ContainElement("synthetic/"))
		})

		DescribeTable("Enable and disable probes if corresponding module is enabled or disabled",
			func(module, probe string) {
				value := renderer.WithEnabledModule(renderer.defaultEnabledModule)
				Expect(value).To(ContainElement(probe))

				value = renderer.WithEnabledModule(module)
				Expect(value).NotTo(ContainElement(probe))
			},
			Entry("Prometheus probe",
				"prometheus",
				"monitoring-and-autoscaling/prometheus"),
			Entry("Trickster probe",
				"prometheus",
				"monitoring-and-autoscaling/trickster"),
			Entry("Prometheus metrics adapter probe",
				"prometheus-metrics-adapter",
				"monitoring-and-autoscaling/prometheus-metrics-adapter"),
		)
	})
})

func newDisabledProbesRenderer(config *Config) *disabledProbesRenderer {
	return &disabledProbesRenderer{
		config:               config,
		defaultMiniDisabled:  true,
		defaultEnabledModule: "random_module",
	}
}

type disabledProbesRenderer struct {
	config               *Config
	defaultMiniDisabled  bool
	defaultEnabledModule string
}

func (c disabledProbesRenderer) WithMiniDisabled(value bool) []string {
	return c.renderValue(c.defaultEnabledModule, value)
}

func (c disabledProbesRenderer) WithEnabledModule(moduleName string) []string {
	return c.renderValue(moduleName, c.defaultMiniDisabled)
}

func (c disabledProbesRenderer) renderValue(moduleName string, miniDisabled bool) []string {
	c.renderTemplates(moduleName, miniDisabled)
	daemonset := c.daemonset()
	return c.findValue(daemonset)
}

func (c disabledProbesRenderer) renderTemplates(moduleName string, miniDisabled bool) {
	c.config.ValuesSetFromYaml("global", fmt.Sprintf(globalValuesFmt, moduleName))
	c.config.ValuesSetFromYaml("upmeter", fmt.Sprintf(upmeterValuesFmt, miniDisabled))
	c.config.HelmRender()
	Expect(c.config.RenderError).ShouldNot(HaveOccurred())
}

func (c disabledProbesRenderer) daemonset() object_store.KubeObject {
	daemonset := c.config.KubernetesResource("DaemonSet", "d8-upmeter", "upmeter-agent")
	Expect(daemonset.Exists()).To(BeTrue())
	return daemonset
}

func (c disabledProbesRenderer) findValue(daemonset object_store.KubeObject) []string {
	envs := daemonset.Field("spec.template.spec.containers.0.env").Array()

	var value string
	for _, upmeterEnv := range envs {
		if upmeterEnv.Get("name").String() == "UPMETER_DISABLED_PROBES" {
			value = upmeterEnv.Get("value").String()
			break
		}
	}
	return strings.Split(value, ",")
}
