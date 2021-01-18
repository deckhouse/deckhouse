package hooks

/*
1. Нет нод в кластере;
2. Есть ноды
3. Есть контролплейн поды
*/

import (
	"encoding/base64"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
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

type testCase struct {
	description string
	input       input
	output      output
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

	hec.ValuesSet("global.clusterConfiguration.kubernetesVersion", caseInput.configVersion)
	hec.BindingContexts.Set(hec.KubeStateSet(b.String()))
}

var _ = Describe("Modules :: controler-plane-manager :: hooks :: get_pki_checksum ::", func() {

	var testingTable = []testCase{
		{
			description: "upgrade: Node version lower than control plane, do not allow to bump effective version and max used version",
			input: input{
				nodeVersions:               []string{"v1.14.3", "v1.14.1", "v1.14.5", "v1.15.2"},
				maxUsedControlPlaneVersion: "1.15",
				configVersion:              "1.16",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.15",
				effectiveVersion:           "1.15",
			},
		},
		{
			description: "upgrade: control plane and nodes are on the same version, allow bumping effective version and max used version",
			input: input{
				nodeVersions:               []string{"v1.15.18", "v1.15.3", "v1.15.5", "v1.15.2"},
				maxUsedControlPlaneVersion: "1.15",
				configVersion:              "1.16",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.16",
				effectiveVersion:           "1.16",
			},
		},
		{
			description: "upgrade: control plane and nodes are on the same version (but kube-scheduler is on a lower version), do not bump effective version and max used version",
			input: input{
				nodeVersions:               []string{"v1.15.18", "v1.15.3", "v1.15.5", "v1.15.2"},
				maxUsedControlPlaneVersion: "1.15",
				configVersion:              "1.16",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.14"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.15",
				effectiveVersion:           "1.15",
			},
		},
		{
			description: "downgrade: control plane and nodes are on the same version, do not lower effective version",
			input: input{
				nodeVersions:               []string{"v1.15.18", "v1.15.3", "v1.15.5", "v1.15.2"},
				maxUsedControlPlaneVersion: "1.15",
				configVersion:              "1.14",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.15",
				effectiveVersion:           "1.15",
			},
		},
		{
			description: "downgrade: nodes are downgraded already, lower effective version",
			input: input{
				nodeVersions:               []string{"v1.14.18", "v1.14.3", "v1.14.5", "v1.14.2"},
				maxUsedControlPlaneVersion: "1.15",
				configVersion:              "1.14",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.15",
				effectiveVersion:           "1.14",
			},
		},
		{
			description: "downgrade: nodes are downgraded already, but configVersion is 2 minor versions lower, lower effective version by one",
			input: input{
				nodeVersions:               []string{"v1.14.18", "v1.14.3", "v1.14.5", "v1.14.2"},
				maxUsedControlPlaneVersion: "1.15",
				configVersion:              "1.13",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.15",
				effectiveVersion:           "1.14",
			},
		},
		{
			description: "downgrade: nodes are downgraded already, but maxUsedControlPlaneVersion does not allow us to downgrade by more than 1",
			input: input{
				nodeVersions:               []string{"v1.14.18", "v1.14.3", "v1.14.5", "v1.14.2"},
				maxUsedControlPlaneVersion: "1.16",
				configVersion:              "1.13",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.16",
				effectiveVersion:           "1.15",
			},
		},
		{
			description: "downgrade: nodes are downgraded already, maxUsedControlPlaneVersion does not allow us to downgrade by more than 1, but we already violating maxUsedControlPlaneVersion",
			input: input{
				nodeVersions:               []string{"v1.14.18", "v1.14.3", "v1.14.5", "v1.14.2"},
				maxUsedControlPlaneVersion: "1.17",
				configVersion:              "1.13",
				controlPlaneVersions:       []string{"1.15", "1.15", "1.15"},
			},
			output: output{
				maxUsedControlPlaneVersion: "1.17",
				effectiveVersion:           "1.15",
			},
		},
	}

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	for _, tCase := range testingTable {
		Context(tCase.description, func() {
			testCase := tCase
			f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

			BeforeEach(func() {
				setStateFromTestCase(f, testCase.input)
				f.RunHook()
			})

			It("", func() {
				Expect(f).To(ExecuteSuccessfully())

				d8ClusterConfigSecret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
				decodedMaxUsedKubernetesVersion, err := base64.StdEncoding.DecodeString(d8ClusterConfigSecret.Field("data.maxUsedControlPlaneKubernetesVersion").String())
				Expect(err).To(BeNil())
				Expect(string(decodedMaxUsedKubernetesVersion)).To(Equal(testCase.output.maxUsedControlPlaneVersion))

				Expect(f.ValuesGet("controlPlaneManager.internal.effectiveKubernetesVersion").String()).To(Equal(testCase.output.effectiveVersion))
			})
		})
	}
})
