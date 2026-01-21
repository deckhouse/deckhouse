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
	"os"
	"path/filepath"
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
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.21.16"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
`
	cniCiliumValues = `
bpfLBMode: "DSR"
internal:
  mode: "Direct"
  masqueradeMode: "BPF"
  hubble:
    certs:
      ca:
        cert: CERT
        key: KEY
      server:
        ca: CA
        key: KEY
        cert: CERT
    settings:
      extendedMetrics:
        enabled: false
        collectors: []
      flowLogs:
        enabled: false
        allowFilterList: []
        denyFilterList: []
        fieldMaskList: []
        fileMaxSizeMB: 10
  egressGatewaysMap:
    myeg:
      name: myeg
      nodeSelector:
        role: worker
      sourceIP:
        mode: VirtualIPAddress
        virtualIPAddress:
          ip: 10.2.2.8
  egressGatewayPolicies:
  - name: egp-dev
    egressGatewayName: myeg
    selectors:
    - podSelector:
        matchLabels:
          app: nginx
    destinationCIDRs:
    - 192.168.0.0/16
    excludedCIDRs:
    - 192.168.3.0/24
  - name: egp-dev-2
    egressGatewayName: myeg
    selectors:
    - podSelector:
        matchLabels:
          app: nginx-2
    destinationCIDRs:
    - 192.168.100.0/16
    excludedCIDRs:
    - 192.168.103.0/24
resourcesManagement:
  mode: VPA
  vpa:
    mode: Auto
    cpu:
      min: "50m"
      max: "2"
    memory:
      min: "256Mi"
      max: "2Gi"
`
)

const hubbleSettings = `
extendedMetrics:
  enabled: true
  collectors:
    - name: drop
      contextOptions: labelsContext=source_ip,source_namespace
    - name: flow
flowLogs:
  enabled: true
  allowFilterList:
    - verdict: ["DROPPED","ERROR"]
    - source_pod: ["kube-system/kube-dns"]
  denyFilterList:
    - source_pod: ["kube-system/"]
    - source_service: ["kube-system/kube-dns"]
  fieldMaskList: ["time","verdict"]
  fileMaxSizeMB: 30
`

func getSubdirs(dir string) ([]string, error) {
	var subdirs []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != dir && filepath.Base(path) == info.Name() {
			subdirs = append(subdirs, info.Name())
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return subdirs, nil
}

const (
	ciliumEETempaltesPath = "/deckhouse/ee/se-plus/modules/021-cni-cilium/templates/"
	ciliumCETempaltesPath = "/deckhouse/modules/021-cni-cilium/templates/"
)

var _ = Describe("Module :: cniCilium :: helm template ::", func() {

	BeforeSuite(func() {
		subDirs, err := getSubdirs(ciliumEETempaltesPath)
		Expect(err).ShouldNot(HaveOccurred())
		for _, subDir := range subDirs {
			err := os.Symlink(ciliumEETempaltesPath+subDir, ciliumCETempaltesPath+subDir)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	AfterSuite(func() {
		subDirs, err := getSubdirs(ciliumEETempaltesPath)
		Expect(err).ShouldNot(HaveOccurred())
		for _, subDir := range subDirs {
			err := os.Remove(ciliumCETempaltesPath + subDir)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	f := SetupHelmConfig(``)

	Context("Cluster with cniCilium", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", cniCiliumValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cegp1 := f.KubernetesGlobalResource("CiliumEgressGatewayPolicy", "d8.egp-dev")
			Expect(cegp1.Exists()).To(BeTrue())
			Expect(cegp1.Field("spec.destinationCIDRs").String()).To(MatchJSON(`["192.168.0.0/16"]`))
			Expect(cegp1.Field("spec.excludedCIDRs").String()).To(MatchJSON(`["192.168.3.0/24"]`))
			Expect(cegp1.Field("spec.selectors").String()).To(MatchJSON(`[{"podSelector": {"matchLabels": {"app": "nginx"}}}]`))
			Expect(cegp1.Field("spec.egressGateway.nodeSelector.matchLabels").String()).To(MatchJSON(`{"egress-gateway.network.deckhouse.io/active-for-myeg": ""}`))

			cegp2 := f.KubernetesGlobalResource("CiliumEgressGatewayPolicy", "d8.egp-dev-2")
			Expect(cegp2.Exists()).To(BeTrue())
			Expect(cegp2.Field("spec.destinationCIDRs").String()).To(MatchJSON(`["192.168.100.0/16"]`))
			Expect(cegp2.Field("spec.excludedCIDRs").String()).To(MatchJSON(`["192.168.103.0/24"]`))
			Expect(cegp2.Field("spec.selectors").String()).To(MatchJSON(`[{"podSelector": {"matchLabels": {"app": "nginx-2"}}}]`))
			Expect(cegp2.Field("spec.egressGateway.nodeSelector.matchLabels").String()).To(MatchJSON(`{"egress-gateway.network.deckhouse.io/active-for-myeg": ""}`))

			ceds := f.KubernetesResource("Daemonset", "d8-cni-cilium", "egress-gateway-agent")
			Expect(ceds.Exists()).To(BeTrue())

			Expect(ceds.Field("spec.template.spec.containers").String()).To(MatchJSON(`[
			{
            "command": [
              "/egress-gateway-agent"
            ],
            "env": [
              {
                "name": "NODE_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "fieldPath": "spec.nodeName"
                  }
                }
              },
              {
                "name": "KUBERNETES_SERVICE_HOST",
                "value": "127.0.0.1"
              },
              {
                "name": "KUBERNETES_SERVICE_PORT",
                "value": "6445"
              }
            ],
            "image": "registry.example.com@imageHash-cniCilium-egressGatewayAgent",
            "livenessProbe": {
              "failureThreshold": 3,
              "httpGet": {
                "host": "127.0.0.1",
                "path": "/healthz",
                "port": 9870
              },
              "initialDelaySeconds": 10,
              "periodSeconds": 10,
              "successThreshold": 1,
              "timeoutSeconds": 1
            },
            "name": "egress-gateway-agent",
            "readinessProbe": {
              "failureThreshold": 3,
              "httpGet": {
                "host": "127.0.0.1",
                "path": "/readyz",
                "port": 9870
              },
              "initialDelaySeconds": 10,
              "periodSeconds": 10,
              "successThreshold": 1,
              "timeoutSeconds": 1
            },
            "resources": {
              "requests": {
                "cpu": "10m",
                "ephemeral-storage": "50Mi",
                "memory": "50Mi"
              }
            },
            "securityContext": {
              "allowPrivilegeEscalation": false,
              "capabilities": {
                "add": [
                  "NET_RAW"
                ],
                "drop": [
                  "ALL"
                ]
              },
              "readOnlyRootFilesystem": true
            }
          }
]`))
		})

	})

	Context("ConfigMap cilium-config rendering (hubble enabled, extended metrics + flow logs)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", cniCiliumValues)

			// Enable cilium-hubble module
			f.ValuesSetFromYaml("global.enabledModules", `[vertical-pod-autoscaler, prometheus, operator-prometheus, cilium-hubble]`)

			// Enable hubble settings
			f.ValuesSetFromYaml("cniCilium.internal.hubble.settings", hubbleSettings)

			f.HelmRender()
		})

		It("Renders ConfigMap cilium-config with expected hubble keys", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config")
			Expect(cm.Exists()).To(BeTrue())

			Expect(cm.Field("data.enable-hubble").String()).To(Equal("true"))
			Expect(cm.Field("data.enable-hubble-open-metrics").String()).To(Equal("true"))
			Expect(cm.Field("data.hubble-metrics-server").String()).To(Equal("127.0.0.1:9091"))

			metrics := cm.Field("data.hubble-metrics").String()
			Expect(metrics).To(ContainSubstring("drop:labelsContext=source_ip,source_namespace"))
			Expect(metrics).To(ContainSubstring("flow"))

			Expect(cm.Field("data.hubble-export-file-path").String()).To(Equal("/var/log/cilium/hubble/flow.log"))
			Expect(cm.Field("data.hubble-export-file-max-size-mb").String()).To(Equal("30"))
			Expect(cm.Field("data.hubble-export-allowlist").String()).To(Equal(`{"verdict":["DROPPED","ERROR"]} {"source_pod":["kube-system/kube-dns"]}`))
			Expect(cm.Field("data.hubble-export-denylist").String()).To(Equal(`{"source_pod":["kube-system/"]} {"source_service":["kube-system/kube-dns"]}`))
			Expect(cm.Field("data.hubble-export-fieldmask").String()).To(Equal("time verdict"))
		})
	})

	Context("ConfigMap cilium-config rendering (hubble module disabled)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", cniCiliumValues)

			f.HelmRender()
		})

		It("Renders ConfigMap cilium-config with disabled cilium-hubble module and without hubble export/metrics keys", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config")
			Expect(cm.Exists()).To(BeTrue())

			Expect(cm.Field("data.enable-hubble").String()).To(Equal("false"))
			Expect(cm.Field("data.hubble-metrics-server").Exists()).To(BeFalse())
			Expect(cm.Field("data.hubble-metrics").Exists()).To(BeFalse())
			Expect(cm.Field("data.hubble-export-file-path").Exists()).To(BeFalse())
			Expect(cm.Field("data.hubble-export-allowlist").Exists()).To(BeFalse())
			Expect(cm.Field("data.hubble-export-denylist").Exists()).To(BeFalse())
			Expect(cm.Field("data.hubble-export-fieldmask").Exists()).To(BeFalse())
		})
	})

	Context("evpn is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", cniCiliumValues)
			// Enable evpn
			f.ValuesSet("cniCilium.evpn.enabled", true)

			f.HelmRender()
		})

		It("Renders evpn manifests", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			evpnS := f.KubernetesResource("StatefulSet", "d8-cni-cilium", "evpn-rr")
			Expect(evpnS.Exists()).To(BeTrue())

			evpnC := f.KubernetesResource("DaemonSet", "d8-cni-cilium", "evpn-client")
			Expect(evpnC.Exists()).To(BeTrue())
		})
	})
})
