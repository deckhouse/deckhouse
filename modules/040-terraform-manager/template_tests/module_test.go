/*
Copyright 2026 Flant JSC

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
	"encoding/json"
	"slices"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
clusterIsBootstrapped: true
enabledModules: ["vertical-pod-autoscaler", "terraform-manager"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterType: Cloud
  cloud:
    provider: Yandex
    prefix: test
  clusterDomain: cluster.local
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  serviceSubnetCIDR: 10.222.0.0/16
modules:
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
  kubernetesVersion: "1.31.0"
`

var _ = Describe("Module :: terraform-manager :: helm template :: in-cluster dhctl rights", func() {
	f := SetupHelmConfig(`{"terraformManager":{"autoConvergerEnabled":true,"autoConvergerPeriod":"1h","internal":{}}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	// A denied read of one of these ModuleConfigs fails the parse, so the converger and the exporter
	// would stop on every tick.
	It("lets the converger and the exporter read every cluster settings ModuleConfig", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for _, name := range []string{"d8:terraform-manager:terraform-auto-converger", "d8:terraform-manager:terraform-state-exporter"} {
			role := f.KubernetesGlobalResource("ClusterRole", name)
			Expect(role.Exists()).To(BeTrue(), name)

			var rules []rbacv1.PolicyRule
			Expect(json.Unmarshal([]byte(role.Field("rules").String()), &rules)).To(Succeed())
			var readable []string
			for _, rule := range rules {
				if slices.Contains(rule.Resources, "moduleconfigs") && slices.Contains(rule.Verbs, "get") {
					readable = append(readable, rule.ResourceNames...)
				}
			}
			Expect(readable).To(ContainElements(config.ClusterSettingsModuleConfigs()), name)
		}
	})
})
