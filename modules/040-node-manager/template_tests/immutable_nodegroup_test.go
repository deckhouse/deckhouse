/*
Copyright 2025 Flant JSC

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
	"encoding/base64"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// Digests as the release publishes them: prefixed, 64 hex characters.
const (
	containerdDigest = "sha256:39a573a08f7562f559aec50882c078ffba3f8eef7d7a479e5db0c021a79135fb"
	cniDigest        = "sha256:fb8005248e6c8f8ca656636174d0db273d35ef460dca527eb6f649127de83f89"
	kubeletDigest    = "sha256:c91f277a75f2daaafe9fc13036ce5e880595a3dfad2e6ecc4acb82994dc609fb"
)

const immutableNodeManagerValues = `
internal:
  capiControllerManagerEnabled: true
  bootstrapTokens:
    immutable-worker: immutabletoken
    worker: mytoken
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
  clusterMasterAddresses: ["10.0.0.1:6443", "10.0.0.2:6443"]
  kubernetesCA: myclusterca
  packagesProxy:
    addresses: ["10.0.0.1:4219"]
    token: rpptoken
  cloudProvider:
    type: dvp
    machineClassKind: ""
    capiClusterKind: "DVPCluster"
    capiClusterAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1"
    capiClusterName: "app"
    capiMachineTemplateKind: "DVPMachineTemplate"
    capiMachineTemplateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1"
    dvp: {}
  nodeGroups:
  - name: immutable-worker
    osType: Immutable
    serializedLabels: ""
    serializedTaints: ""
    nodeType: CloudEphemeral
    kubernetesVersion: "1.34"
    instanceClass:
      rootDisk:
        size: 20Gi
        storageClass: linstor
        image:
          kind: ClusterVirtualImage
          name: olcedar
      virtualMachine:
        virtualMachineClassName: generic
        bootloader: EFI
        memory:
          size: 4Gi
        cpu:
          cores: 2
          coreFraction: 100%
    cloudInstances:
      classReference:
        kind: DVPInstanceClass
        name: immutable-worker
      maxPerZone: 3
      minPerZone: 1
      zones:
      - zonea
  - name: worker
    serializedLabels: ""
    serializedTaints: ""
    nodeType: CloudEphemeral
    kubernetesVersion: "1.34"
    instanceClass:
      rootDisk:
        size: 20Gi
        storageClass: linstor
        image:
          kind: ClusterVirtualImage
          name: olcedar
      virtualMachine:
        virtualMachineClassName: generic
        bootloader: EFI
        memory:
          size: 4Gi
        cpu:
          cores: 2
          coreFraction: 100%
    cloudInstances:
      classReference:
        kind: DVPInstanceClass
        name: worker
      maxPerZone: 3
      minPerZone: 1
      zones:
      - zonea
  machineControllerManagerEnabled: false
`

// bootstrapValues extracts every `value: <base64>` of the rendered bootstrap
// Secrets. Their names carry an instance-class checksum, so the manifests are
// matched by content instead of by name.
func bootstrapValues(manifests string) []string {
	re := regexp.MustCompile(`(?m)^\s*value:\s+(\S+)\s*$`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(manifests, -1) {
		decoded, err := base64.StdEncoding.DecodeString(m[1])
		Expect(err).ShouldNot(HaveOccurred())
		out = append(out, string(decoded))
	}
	return out
}

var _ = Describe("Module :: node-manager :: helm template :: immutable NodeGroup", func() {
	f := SetupHelmConfig(``)
	rendered := map[string]string{}

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.discovery.kubernetesVersion", "1.34.9")
		f.ValuesSet("global.discovery.clusterDomain", "cluster.local")
		f.ValuesSet("global.discovery.clusterDNSAddress", "10.222.0.10")
		f.ValuesSet("global.modulesImages", GetModulesImages())
		// Real digests already carry the sha256: prefix the NodeConfig schema
		// demands, so the rendered value must not add one.
		f.ValuesSet("global.modulesImages.digests.registrypackages.containerdSysext224", containerdDigest)
		f.ValuesSet("global.modulesImages.digests.registrypackages.kubernetesCniSysext162", cniDigest)
		f.ValuesSet("global.modulesImages.digests.registrypackages.kubeletSysext1349", kubeletDigest)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+immutableNodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender(WithFilteredRenderOutput(rendered, []string{"node-group/node-group.yaml"}))
	})

	It("bootstraps the immutable group from a NodeConfig and the mutable one from bashible", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		var manifests []string
		for _, m := range rendered {
			manifests = append(manifests, m)
		}

		var immutable, mutable string
		for _, v := range bootstrapValues(strings.Join(manifests, "\n")) {
			if strings.Contains(v, "kind: NodeConfig") {
				immutable = v
				continue
			}
			mutable = v
		}

		Expect(immutable).ShouldNot(BeEmpty(), "the immutable group must get a NodeConfig userdata")
		Expect(mutable).Should(ContainSubstring("bootstrap.sh"), "the mutable group must keep the bashible userdata")

		Expect(immutable).Should(ContainSubstring("nodeName: __NODE_NAME__"))
		Expect(immutable).Should(ContainSubstring("/config/config.ign"))
		Expect(immutable).Should(ContainSubstring("__INSTALL_DISK__"))
		Expect(immutable).Should(ContainSubstring("externalCloudProvider: true"))
		Expect(immutable).Should(ContainSubstring("bootstrapToken: immutabletoken"))
		Expect(immutable).Should(ContainSubstring("clusterDNS: [\"10.222.0.10\"]"))
		Expect(immutable).Should(ContainSubstring("- \"https://10.0.0.1:6443\""))
		// The kubelet extension must follow the effective cluster version.
		Expect(immutable).Should(ContainSubstring("digest: " + kubeletDigest))
		Expect(immutable).Should(ContainSubstring("digest: " + containerdDigest))
		Expect(immutable).Should(ContainSubstring("digest: " + cniDigest))
		Expect(immutable).ShouldNot(ContainSubstring("sha256:sha256:"))
		Expect(immutable).Should(ContainSubstring("caCert: " + base64.StdEncoding.EncodeToString([]byte("myclusterca"))))
		Expect(immutable).Should(ContainSubstring("registryPackagesProxyAccessTokenB64: " + base64.StdEncoding.EncodeToString([]byte("rpptoken"))))
	})
})
