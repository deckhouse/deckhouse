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
    immutable-static: statictoken
    immutable-permanent: permanenttoken
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
    systemType: Immutable
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
  - name: immutable-static
    systemType: Immutable
    serializedLabels: ""
    serializedTaints: ""
    nodeType: Static
    kubernetesVersion: "1.34"
  - name: immutable-permanent
    systemType: Immutable
    serializedLabels: ""
    serializedTaints: ""
    nodeType: CloudPermanent
    kubernetesVersion: "1.34"
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

	It("hands every manually bootstrapped immutable group the same NodeConfig userdata", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		// Static and cloud permanent groups bootstrap their nodes by hand and
		// are served by the same Secret.
		for _, ngName := range []string{"immutable-static", "immutable-permanent"} {
			secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "manual-bootstrap-for-"+ngName)
			Expect(secret.Exists()).To(BeTrue(), ngName)
			userdata, err := base64.StdEncoding.DecodeString(secret.Field("data.cloud-config").String())
			Expect(err).ShouldNot(HaveOccurred(), ngName)
			Expect(string(userdata)).Should(ContainSubstring("kind: NodeConfig"), ngName)
			// bashible's bootstrap script has no business on such a node.
			Expect(secret.Field(`data.bootstrap\.sh`).Exists()).To(BeFalse(), ngName)
		}
	})

	It("renders no helm bootstrap secret for a CAPI immutable group and keeps bashible for the mutable one", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		var manifests []string
		for _, m := range rendered {
			manifests = append(manifests, m)
		}

		// A CAPI immutable group boots from a per-machine NodeBootstrapConfig the
		// node-controller bootstrap provider renders (the MachineDeployment points
		// at it through bootstrap.configRef), so helm emits no group-wide bootstrap
		// secret for it: none of the value: secrets carry a NodeConfig.
		var mutable string
		for _, v := range bootstrapValues(strings.Join(manifests, "\n")) {
			Expect(v).ShouldNot(ContainSubstring("kind: NodeConfig"),
				"a CAPI immutable group must not get a helm-rendered NodeConfig bootstrap secret")
			mutable = v
		}

		// The mutable group keeps its bashible userdata.
		Expect(mutable).ShouldNot(BeEmpty(), "the mutable group must still get a bashible bootstrap secret")
		Expect(mutable).Should(ContainSubstring("bootstrap.sh"))
	})
})
