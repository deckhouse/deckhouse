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
	"os"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/values/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const fencingDataInternalValues = `
internal:
  capiControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  capsControllerManagerWebhookCert:
    ca: string
    key: string
    crt: string
  nodeControllerWebhookCert:
    ca: string
    key: string
    crt: string
  instancePrefix: myprefix
  clusterMasterAddresses: ["10.0.0.1:6443"]
`

// builtinProfileNames are the names of the built-in profiles.
var builtinProfileNames = []string{"critical", "normal", "slow"}

var _ = Describe("Module :: node-manager :: helm template :: fencing profiles and configuration", func() {
	f := SetupHelmConfig(``)

	render := func(fencingValues string, moduleConfigFencing ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+fencingDataInternalValues)
		for _, settings := range moduleConfigFencing {
			f.ValuesSetFromYaml("nodeManager.fencing", settings)
		}
		f.ValuesSetFromYaml("nodeManager.internal.fencing", fencingValues)
		f.ApplyOpenAPIDefaults()
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	agentConfig := func() map[string]any {
		cm := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "fencing-agent-config")
		Expect(cm.Exists()).To(BeTrue())
		Expect(cm.Field("data").Map()).To(HaveLen(1))

		var config map[string]any
		Expect(json.Unmarshal([]byte(cm.Field("data.config\\.json").String()), &config)).To(Succeed())
		return config
	}

	Context("every node group is Disabled", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Disabled}}`)
		})

		It("ships the three built-in profiles and no agent configuration", func() {
			for _, name := range []string{"critical", "normal", "slow"} {
				Expect(f.KubernetesGlobalResource("FencingSLAProfile", name).Exists()).To(BeTrue(), name)
			}
			Expect(f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "fencing-agent-config").Exists()).To(BeFalse())
		})
	})

	Context("built-in profiles", func() {
		BeforeEach(func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
		})

		It("are exactly critical, normal and slow, owned by Deckhouse", func() {
			files, err := filepath.Glob("../templates/fencing-agent/sla-profiles/*.yaml")
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(3))

			for _, name := range builtinProfileNames {
				Expect(f.KubernetesGlobalResource("FencingSLAProfile", name).Field("metadata.labels.heritage").String()).To(Equal("deckhouse"), name)
			}
		})

		It("have no field of correlation, the brake or the eviction limit, and no field of fencing 1.0", func() {
			for _, name := range builtinProfileNames {
				spec := f.KubernetesGlobalResource("FencingSLAProfile", name).Field("spec").Map()
				for _, removed := range []string{"reactionGoal", "detectionWindowTarget", "fallback", "evacuation", "minFailures", "window", "massFailureThreshold", "maxConcurrentEvictions"} {
					Expect(spec).NotTo(HaveKey(removed), name)
				}
			}
		})

		It("have no common timings of the agent: they are in the agent configuration", func() {
			for _, name := range builtinProfileNames {
				profile := f.KubernetesGlobalResource("FencingSLAProfile", name)
				Expect(profile.Field("spec.proxy").Exists()).To(BeFalse(), name)
				Expect(profile.Field("spec.poll").Map()).To(HaveLen(1), name)
				Expect(profile.Field("spec.poll").Map()).To(HaveKey("witnessFresherBy"), name)
			}
		})

		It("have different checksums that are 64 hex characters", func() {
			sums := map[string]bool{}
			for _, name := range builtinProfileNames {
				sum := f.KubernetesGlobalResource("FencingSLAProfile", name).Field(`metadata.annotations.node-manager\.deckhouse\.io/fencing-profile-checksum`).String()
				Expect(sum).To(MatchRegexp(`^[0-9a-f]{64}$`), name)
				sums[sum] = true
			}
			Expect(sums).To(HaveLen(3))
		})
	})

	Context("agent configuration", func() {
		BeforeEach(func() {
			render(`
nodeGroups:
  a: {mode: Slow}
  b: {mode: Normal}
  c: {mode: Disabled}
`)
		})

		It("holds the normalized modes, the memberlist profile and the defaults of the internal values", func() {
			config := agentConfig()

			Expect(config["clusterUUID"]).To(Equal("f49dd1c3-a63a-4565-a06c-625e35587eab"))
			Expect(config["nodeGroups"]).To(Equal(map[string]any{
				"a": map[string]any{"mode": "Slow"},
				"b": map[string]any{"mode": "Normal"},
				"c": map[string]any{"mode": "Disabled"},
			}))
			Expect(config["memberlistProfile"]).To(Equal("normal"))
			Expect(config["minFullPollInterval"]).To(Equal("10s"))
			Expect(config["peerListRefreshInterval"]).To(Equal("5m"))
			Expect(config["systemNetworkName"]).To(Equal(""))
		})

		It("holds only the keys the agent and the controller know", func() {
			Expect(agentConfig()).To(HaveLen(8))
		})

		It("holds the checksum of every profile, equal byte by byte to the annotation of the profile", func() {
			sums, ok := agentConfig()["profileChecksums"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(sums).To(HaveLen(3))

			for _, name := range builtinProfileNames {
				annotation := f.KubernetesGlobalResource("FencingSLAProfile", name).Field(`metadata.annotations.node-manager\.deckhouse\.io/fencing-profile-checksum`).String()
				Expect(sums[name]).To(Equal(annotation), name)
			}
		})

		It("is neither mounted nor watched by pod-reloader", func() {
			cm := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "fencing-agent-config")
			Expect(cm.Field("metadata.annotations").Exists()).To(BeFalse())
		})
	})

	Context("common timings in the internal values", func() {
		BeforeEach(func() {
			render(`
nodeGroups: {a: {mode: Slow}}
commonTimings: {roundTimeout: 200ms, confirmTimeout: 700ms, fastReadAfter: 1.5s}
`)
		})

		It("go to the agent configuration as one block", func() {
			Expect(agentConfig()["commonTimings"]).To(Equal(map[string]any{
				"roundTimeout": "200ms", "confirmTimeout": "700ms", "fastReadAfter": "1.5s",
			}))
		})
	})

	DescribeTable("the memberlist profile is the fastest of the armed groups",
		func(fencing, wantProfile string) {
			render(fencing)
			Expect(agentConfig()["memberlistProfile"]).To(Equal(wantProfile))
		},
		Entry("Critical wins", `nodeGroups: {a: {mode: Slow}, b: {mode: Critical}, c: {mode: Normal}}`, "critical"),
		Entry("Normal wins over Slow", `nodeGroups: {a: {mode: Slow}, b: {mode: Normal}}`, "normal"),
		Entry("Slow alone", `nodeGroups: {a: {mode: Slow}, b: {mode: Disabled}}`, "slow"),
	)

	Context("the name of a SystemNetwork is set in the ModuleConfig", func() {
		// The setting exists only in the config-values of EE, FE and CSE: the schema of CE refuses it.
		var ceSchema *validation.SchemaStorage
		BeforeEach(func() {
			config, err := os.ReadFile("../../../ee/modules/040-node-manager/openapi/config-values.yaml")
			Expect(err).NotTo(HaveOccurred())
			values, err := os.ReadFile("../openapi/values.yaml")
			Expect(err).NotTo(HaveOccurred())
			eeSchema, err := validation.NewSchemaStorage(config, values)
			Expect(err).NotTo(HaveOccurred())

			ceSchema = f.ValuesValidator.ModuleSchemaStorages["nodeManager"]
			f.ValuesValidator.ModuleSchemaStorages["nodeManager"] = eeSchema
		})
		AfterEach(func() {
			f.ValuesValidator.ModuleSchemaStorages["nodeManager"] = ceSchema
		})

		It("is copied as it is, also when the network does not exist in the cluster", func() {
			render(`nodeGroups: {a: {mode: Slow}}`, `systemNetworkFallback: {systemNetworkName: sysnet-removed}`)

			Expect(agentConfig()["systemNetworkName"]).To(Equal("sysnet-removed"))
		})
	})

	Context("a group runs on Deckhouse Engine", func() {
		It("carries the engine flag into the agent configuration", func() {
			render(`nodeGroups: {a: {mode: Slow, engine: true}, b: {mode: Slow, engine: false}}`)
			Expect(agentConfig()["nodeGroups"]).To(Equal(map[string]any{
				"a": map[string]any{"mode": "Slow", "engine": true},
				"b": map[string]any{"mode": "Slow", "engine": false},
			}))
		})
	})

	Context("a group changes its mode", func() {
		It("changes the ConfigMap only", func() {
			render(`nodeGroups: {a: {mode: Slow}, b: {mode: Normal}}`)
			before := f.KubernetesGlobalResource("FencingSLAProfile", "normal").ToYaml()
			Expect(agentConfig()["nodeGroups"]).To(HaveKeyWithValue("a", map[string]any{"mode": "Slow"}))

			render(`nodeGroups: {a: {mode: Critical}, b: {mode: Normal}}`)
			Expect(agentConfig()["nodeGroups"]).To(HaveKeyWithValue("a", map[string]any{"mode": "Critical"}))
			Expect(f.KubernetesGlobalResource("FencingSLAProfile", "normal").ToYaml()).To(Equal(before))
		})
	})
})
