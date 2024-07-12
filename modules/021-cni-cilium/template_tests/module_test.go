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
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus", "operator-prometheus-crd"]
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
	ciliumEETempaltesPath = "/deckhouse/ee/modules/021-cni-cilium/templates/"
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
			cegp := f.KubernetesGlobalResource("CiliumEgressGatewayPolicy", "d8.myeg")
			Expect(cegp.Exists()).To(BeTrue())

			Expect(cegp.Field("spec.excludedCIDRs").String()).To(MatchJSON(`["192.168.0.0/16"]`))

			Expect(cegp.Field("spec.selectors").String()).To(MatchJSON(`[{"podSelector": {"matchLabels": {"app": "nginx"}}}]`))

			Expect(cegp.Field("spec.egressGateway.nodeSelector.matchLabels").String()).To(MatchJSON(`{"egress-gateway.network.deckhouse.io/active-for-myeg": ""}`))

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
})
