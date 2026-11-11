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
	"os/exec"
	"strings"
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
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    kind: ClusterConfiguration
    clusterType: Static
    clusterDomain: "cluster.local"
    kubernetesVersion: "1.33"
    serviceSubnetCIDR: "10.222.0.0/16"
    podSubnetCIDR: "10.111.0.0/16"
  modules:
    placement: {}
  discovery:
    kubernetesVersion: 1.33.5
    clusterDomain: "cluster.local"
    d8SpecificNodeCountByRole:
      worker: 3
      master: 3
`
	moduleValues = `
ntpServers: ["pool.ntp.org", "ntp.ubuntu.com"]
`
)

func expectValidBash(script string) {
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "invalid bash: %s", output)
}

func expectEarlyTimeSyncCleanup(f *Config) {
	Expect(f.RenderError).ShouldNot(HaveOccurred())
	earlyTimeSync := f.KubernetesGlobalResource("NodeGroupConfiguration", "early-time-sync-before-kubelet.sh")
	Expect(earlyTimeSync.Exists()).To(BeTrue())
	// Without the ConfigMap node-controller gives Engine masters no NTP servers.
	Expect(f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "chrony-early-time-sync").Exists()).To(BeFalse())

	content := earlyTimeSync.Field("spec.content").String()
	expectValidBash(content)
	Expect(content).To(ContainSubstring("rm -f /etc/systemd/system/kubelet.service.d/20-d8-early-time-sync.conf"))
	Expect(content).To(ContainSubstring("rm -f /etc/systemd/system/nodelet.service.d/20-d8-early-time-sync.conf"))
	Expect(content).NotTo(ContainSubstring("bb-sync-file"))
	Expect(content).To(ContainSubstring("/var/run/node-exporter-textfile/d8_early_time_sync.prom"))
	Expect(content).To(ContainSubstring("systemctl daemon-reload"))
}

var _ = Describe("Module :: chrony :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-chrony")
			registrySecret := f.KubernetesResource("Secret", "d8-chrony", "deckhouse-registry")
			earlyTimeSync := f.KubernetesGlobalResource("NodeGroupConfiguration", "early-time-sync-before-kubelet.sh")

			chronyDaemonSetTest := f.KubernetesResource("DaemonSet", "d8-chrony", "chrony")
			chronyMasterDaemonsetTest := f.KubernetesResource("DaemonSet", "d8-chrony", "chrony-master")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(earlyTimeSync.Exists()).To(BeTrue())
			Expect(earlyTimeSync.Field("spec.nodeGroups").String()).To(MatchJSON(`["master"]`))

			content := earlyTimeSync.Field("spec.content").String()
			expectValidBash(content)
			Expect(content).To(ContainSubstring("\n[Unit]\nAfter=nodelet.service\n"))
			Expect(content).NotTo(ContainSubstring("Wants=nodelet.service"))
			Expect(content).NotTo(ContainSubstring("\nRequires="))
			Expect(content).To(ContainSubstring("'d8_early_time_sync_success 0'"))
			Expect(content).To(ContainSubstring("systemctl is-failed -q nodelet.service"))
			Expect(content).To(ContainSubstring("bb-sync-file /etc/systemd/system/nodelet.service.d/20-d8-early-time-sync.conf"))
			Expect(content).To(ContainSubstring("\nEnvironment=\"NODELET_BOOT_NTP_SERVERS=pool.ntp.org ntp.ubuntu.com\"\n"))
			Expect(content).To(ContainSubstring("\nEnvironment=NODELET_BOOT_NTP_TIMEOUT=30s\nTimeoutStartSec=35\n"))
			Expect(content).NotTo(ContainSubstring("-sync-clock"))

			engineTimeSync := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "chrony-early-time-sync")
			Expect(engineTimeSync.Exists()).To(BeTrue())
			Expect(engineTimeSync.Field("data.servers").String()).To(MatchJSON(`["pool.ntp.org","ntp.ubuntu.com"]`))

			Expect(chronyDaemonSetTest.Exists()).To(BeTrue())
			Expect(chronyMasterDaemonsetTest.Exists()).To(BeTrue())

			Expect(chronyDaemonSetTest.Field("spec.template.spec.containers.2.env.1.value").String()).To(
				ContainSubstring("upstream: http://127.0.0.1:4245/\n  path: /healthz"),
			)
			Expect(chronyMasterDaemonsetTest.Field("spec.template.spec.containers.2.env.1.value").String()).To(
				ContainSubstring("upstream: http://127.0.0.1:4245/\n  path: /healthz"),
			)

			Expect(chronyDaemonSetTest.Field("spec.template.spec.containers.0.env").String()).To(MatchJSON(`
        [
		  {
            "name": "PATH",
            "value": "/opt/chrony-static/bin"
          },
          {
            "name": "NTP_ROLE",
            "value": "sink"
          },
          {
            "name": "NTP_SERVERS",
            "value": "pool.ntp.org. ntp.ubuntu.com."
          },
          {
            "name": "CHRONY_MASTERS_SERVICE",
            "value": "chrony-masters.d8-chrony.svc.cluster.local"
          },
          {
            "name": "HOST_IP",
            "valueFrom": {
              "fieldRef": {
                "fieldPath": "status.hostIP"
              }
            }
          }
        ]
`))
			Expect(chronyMasterDaemonsetTest.Field("spec.template.spec.containers.0.env").String()).To(MatchJSON(`
        [
          {
            "name": "PATH",
            "value": "/opt/chrony-static/bin"
          },
          {
            "name": "NTP_ROLE",
            "value": "source"
          },
          {
            "name": "NTP_SERVERS",
            "value": "pool.ntp.org. ntp.ubuntu.com."
          },
          {
            "name": "HOST_IP",
            "valueFrom": {
              "fieldRef": {
                "fieldPath": "status.hostIP"
              }
            }
          }
        ]
`))
		})

		It("Namespace must opt into the restricted policy with exception checks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-chrony")
			Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/pod-policy`).String()).To(Equal("restricted"))
			Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/enable-security-policy-check`).String()).To(Equal("true"))
		})

		It("SecurityPolicyException must not be rendered without its API", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-chrony", "chrony").Exists()).To(BeFalse())
		})
	})

	Context("SecurityPolicyException", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", moduleValues)
			f.HelmRender()
		})

		for _, name := range []string{"chrony", "chrony-master"} {
			It(name+" must be covered by the exception it points to", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				ds := f.KubernetesResource("DaemonSet", "d8-chrony", name)
				speName := ds.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()
				spe := f.KubernetesResource("SecurityPolicyException", "d8-chrony", speName)
				Expect(spe.Exists()).To(BeTrue())

				allowedCapabilities := map[string]bool{}
				for _, capability := range spe.Field("spec.securityContext.capabilities.allowedValues.add").Array() {
					allowedCapabilities[capability.String()] = true
				}
				allowedPorts := map[string]bool{}
				for _, port := range spe.Field("spec.network.hostPorts").Array() {
					allowedPorts[port.Get("port").String()+"/"+port.Get("protocol").String()] = true
				}
				// The exception matches a hostPath only when its readOnly equals the mount's.
				allowedHostPaths := map[string]bool{}
				for _, entry := range spe.Field("spec.volumes.hostPath.allowedValues").Array() {
					allowedHostPaths[entry.Get("path").String()] = entry.Get("readOnly").Bool()
				}

				mountReadOnly := map[string]bool{}
				for _, container := range ds.Field("spec.template.spec.containers").Array() {
					for _, capability := range container.Get("securityContext.capabilities.add").Array() {
						Expect(allowedCapabilities).To(HaveKey(capability.String()), "container %s", container.Get("name"))
					}
					// The pod runs in the host network, so every container port is a host port.
					for _, port := range container.Get("ports").Array() {
						protocol := port.Get("protocol").String()
						if protocol == "" {
							protocol = "TCP"
						}
						Expect(allowedPorts).To(HaveKey(port.Get("containerPort").String()+"/"+protocol), "container %s", container.Get("name"))
					}
					for _, mount := range container.Get("volumeMounts").Array() {
						mountReadOnly[mount.Get("name").String()] = mount.Get("readOnly").Bool()
					}
				}
				for _, volume := range ds.Field("spec.template.spec.volumes").Array() {
					hostPath := volume.Get("hostPath.path")
					if !hostPath.Exists() {
						continue
					}
					Expect(allowedHostPaths).To(HaveKeyWithValue(hostPath.String(), mountReadOnly[volume.Get("name").String()]))
				}
			})
		}
	})
	Context("Custom early time sync timeout", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", moduleValues)
			f.ValuesSet("chrony.earlyTimeSync.timeoutSeconds", 60)
			f.HelmRender()
		})

		It("Waits for the configured time", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			content := f.KubernetesGlobalResource("NodeGroupConfiguration", "early-time-sync-before-kubelet.sh").Field("spec.content").String()
			Expect(content).To(ContainSubstring("\nEnvironment=NODELET_BOOT_NTP_TIMEOUT=60s\nTimeoutStartSec=65\n"))
		})
	})

	Context("Early time sync disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", moduleValues)
			f.ValuesSet("chrony.earlyTimeSync.enabled", false)
			f.HelmRender()
		})

		It("Removes the boot-time synchronization settings", func() {
			expectEarlyTimeSyncCleanup(f)
		})
	})

	Context("Empty ntpServers", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", `ntpServers: []`)
			f.HelmRender()
		})

		It("Removes the boot-time synchronization settings", func() {
			expectEarlyTimeSyncCleanup(f)
		})
	})

	Context("ntpServers the boot-time sync cannot take", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", `ntpServers: ["2001:db8::1", "pool.ntp.org:123", "pool.ntp.org\nEOF", "ntp.example.com"]`)
			f.HelmRender()
		})

		It("Skips them on both kinds of nodes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			content := f.KubernetesGlobalResource("NodeGroupConfiguration", "early-time-sync-before-kubelet.sh").Field("spec.content").String()
			Expect(content).To(ContainSubstring("\nEnvironment=\"NODELET_BOOT_NTP_SERVERS=ntp.example.com\"\n"))
			servers := f.KubernetesResource("ConfigMap", "d8-cloud-instance-manager", "chrony-early-time-sync").Field("data.servers").String()
			Expect(servers).To(Equal(`["ntp.example.com"]`))
		})
	})

	Context("No ntpServers the boot-time sync can take", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("chrony", `ntpServers: ["2001:db8::1"]`)
			f.HelmRender()
		})

		It("Removes the boot-time synchronization settings", func() {
			expectEarlyTimeSyncCleanup(f)
		})
	})
})
