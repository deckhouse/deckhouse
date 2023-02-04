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

package hooks

/*
1. No nodes in the cluster;
2. Some nodes exist;
3. There are Pods with control-plane.
*/

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type input struct {
	nodeVersions               []string
	maxUsedControlPlaneVersion string
	configVersion              string
	controlPlaneVersions       []string
}

type output struct {
	maxUsedControlPlaneVersion string
	effectiveVersion           string
}

func setStateFromTestCase(hec *HookExecutionConfig, caseInput input) {
	const nodeTemplate = `
---
apiVersion: v1
kind: Node
metadata:
  name: test-<<INDEX>>
status:
  nodeInfo:
    kubeletVersion: "<<PLACEHOLDER>>"
`

	const kubeApiserverPodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver-kube-master-<<INDEX>>
  namespace: kube-system
  labels:
    component: kube-apiserver
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: "<<PLACEHOLDER>>"
`

	const kubeControllerManagerPodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager-kube-master-<<INDEX>>
  namespace: kube-system
  labels:
    component: kube-controller-manager
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: "<<PLACEHOLDER>>"
`

	const kubeSchedulerPodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler-kube-master-<<INDEX>>
  namespace: kube-system
  labels:
    component: kube-scheduler
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: "<<PLACEHOLDER>>"
`

	const d8ConfigurationSecretTemplate = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  maxUsedControlPlaneKubernetesVersion: "<<PLACEHOLDER_B64>>"
`

	var b strings.Builder

	for index, nodeVersion := range caseInput.nodeVersions {
		nodeManifest := strings.ReplaceAll(nodeTemplate, "<<PLACEHOLDER>>", nodeVersion)
		nodeManifest = strings.ReplaceAll(nodeManifest, "<<INDEX>>", strconv.Itoa(index))

		b.WriteString(nodeManifest)
	}

	for index, controlPlaneVersion := range caseInput.controlPlaneVersions {
		kubeApiserverManifest := strings.ReplaceAll(kubeApiserverPodTemplate, "<<PLACEHOLDER>>", controlPlaneVersion)
		kubeApiserverManifest = strings.ReplaceAll(kubeApiserverManifest, "<<INDEX>>", strconv.Itoa(index))
		b.WriteString(kubeApiserverManifest)

		kubeControllerManager := strings.ReplaceAll(kubeControllerManagerPodTemplate, "<<PLACEHOLDER>>", controlPlaneVersion)
		kubeControllerManager = strings.ReplaceAll(kubeControllerManager, "<<INDEX>>", strconv.Itoa(index))
		b.WriteString(kubeControllerManager)

		kubeSchedulerManifest := strings.ReplaceAll(kubeSchedulerPodTemplate, "<<PLACEHOLDER>>", controlPlaneVersion)
		kubeSchedulerManifest = strings.ReplaceAll(kubeSchedulerManifest, "<<INDEX>>", strconv.Itoa(index))
		b.WriteString(kubeSchedulerManifest)
	}

	b.WriteString(strings.ReplaceAll(d8ConfigurationSecretTemplate, "<<PLACEHOLDER_B64>>", base64.StdEncoding.EncodeToString([]byte(caseInput.maxUsedControlPlaneVersion))))

	clusterConf := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
cloud:
  prefix: sandbox
  provider: OpenStack
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Docker
kind: ClusterConfiguration
kubernetesVersion: "%s"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`, caseInput.configVersion)
	hec.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConf))
	hec.BindingContexts.Set(hec.KubeStateSet(b.String()))
}

var _ = Describe("Modules :: control-plane-manager :: hooks :: get_pki_checksum ::", func() {
	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(BeEquivalentTo("global.clusterConfiguration.kubernetesVersion required"))
		})
	})

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		DescribeTable("version change",
			func(in input, out output) {
				setStateFromTestCase(f, in)
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				d8ClusterConfigSecret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
				decodedMaxUsedKubernetesVersion, err := base64.StdEncoding.DecodeString(d8ClusterConfigSecret.Field("data.maxUsedControlPlaneKubernetesVersion").String())
				Expect(err).To(BeNil())
				Expect(string(decodedMaxUsedKubernetesVersion)).To(Equal(out.maxUsedControlPlaneVersion))

				Expect(f.ValuesGet("controlPlaneManager.internal.effectiveKubernetesVersion").String()).To(Equal(out.effectiveVersion))
			},
			Entry("upgrade: Node version lower than control plane, do not allow to bump effective version and max used version",
				input{
					nodeVersions:               []string{"v1.22.3", "v1.22.1", "v1.22.5", "v1.23.2"},
					maxUsedControlPlaneVersion: "1.23",
					configVersion:              "1.24",
					controlPlaneVersions:       []string{"1.23", "1.23", "1.23"},
				},
				output{
					maxUsedControlPlaneVersion: "1.23",
					effectiveVersion:           "1.23",
				},
			),
			Entry("upgrade: control plane and nodes are on the same version, allow bumping effective version and max used version", input{
				nodeVersions:               []string{"v1.23.10", "v1.23.3", "v1.23.5", "v1.23.2"},
				maxUsedControlPlaneVersion: "1.23",
				configVersion:              "1.24",
				controlPlaneVersions:       []string{"1.23", "1.23", "1.23"},
			},
				output{
					maxUsedControlPlaneVersion: "1.24",
					effectiveVersion:           "1.24",
				},
			),
			Entry("upgrade: control plane and nodes are on the same version (but kube-scheduler is on a lower version), do not bump effective version and max used version",
				input{
					nodeVersions:               []string{"v1.23.10", "v1.23.3", "v1.23.5", "v1.23.2"},
					maxUsedControlPlaneVersion: "1.23",
					configVersion:              "1.24",
					controlPlaneVersions:       []string{"1.23", "1.23", "1.22"},
				},
				output{
					maxUsedControlPlaneVersion: "1.23",
					effectiveVersion:           "1.23",
				},
			),
			Entry("downgrade: control plane and nodes are on the same version, do not lower effective version",
				input{
					nodeVersions:               []string{"v1.23.10", "v1.23.3", "v1.23.5", "v1.23.2"},
					maxUsedControlPlaneVersion: "1.23",
					configVersion:              "1.22",
					controlPlaneVersions:       []string{"1.23", "1.23", "1.23"},
				},
				output{
					maxUsedControlPlaneVersion: "1.23",
					effectiveVersion:           "1.23",
				},
			),
			Entry("downgrade: nodes are downgraded already, lower effective version",
				input{
					nodeVersions:               []string{"v1.23.10", "v1.23.3", "v1.23.5", "v1.23.2"},
					maxUsedControlPlaneVersion: "1.24",
					configVersion:              "1.22",
					controlPlaneVersions:       []string{"1.24", "1.24", "1.24"},
				},
				output{
					maxUsedControlPlaneVersion: "1.24",
					effectiveVersion:           "1.23",
				},
			),
			Entry("downgrade: nodes are downgraded already, but configVersion is 2 minor versions lower, lower effective version by one",
				input{
					nodeVersions:               []string{"v1.23.10", "v1.23.3", "v1.23.5", "v1.23.2"},
					maxUsedControlPlaneVersion: "1.24",
					configVersion:              "1.22",
					controlPlaneVersions:       []string{"1.24", "1.24", "1.24"},
				},
				output{
					maxUsedControlPlaneVersion: "1.24",
					effectiveVersion:           "1.23",
				},
			),
			Entry("downgrade: nodes are downgraded already, but maxUsedControlPlaneVersion does not allow us to downgrade by more than 1",
				input{
					nodeVersions:               []string{"v1.22.4", "v1.22.3", "v1.22.5", "v1.22.2"},
					maxUsedControlPlaneVersion: "1.24",
					configVersion:              "1.22",
					controlPlaneVersions:       []string{"1.23", "1.23", "1.23"},
				},
				output{
					maxUsedControlPlaneVersion: "1.24",
					effectiveVersion:           "1.23",
				},
			),
			Entry("downgrade: nodes are downgraded already, maxUsedControlPlaneVersion does not allow us to downgrade by more than 1, but we already violating maxUsedControlPlaneVersion",
				input{
					nodeVersions:               []string{"v1.22.4", "v1.22.3", "v1.22.5", "v1.22.2"},
					maxUsedControlPlaneVersion: "1.25",
					configVersion:              "1.23",
					controlPlaneVersions:       []string{"1.23", "1.23", "1.23"},
				},
				output{
					maxUsedControlPlaneVersion: "1.25",
					effectiveVersion:           "1.23",
				},
			),
		)
	})
})
