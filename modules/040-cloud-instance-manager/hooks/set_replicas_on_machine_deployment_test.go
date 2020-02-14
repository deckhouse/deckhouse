package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: set_replicas_on_machine_deployment ::", func() {
	const (
		stateCIGs = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: cig1
spec:
  maxInstancesPerZone: 2
  minInstancesPerZone: 5 # $ig_min_instances -ge $ig_max_instances
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: cig20
spec:
  maxInstancesPerZone: 4
  minInstancesPerZone: 3 # "$replicas" == "null"
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: cig21
spec:
  maxInstancesPerZone: 4
  minInstancesPerZone: 3 # $replicas -eq 0
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: cig3
spec:
  maxInstancesPerZone: 10
  minInstancesPerZone: 6 # $replicas -le $ig_min_instances
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: cig4
spec:
  maxInstancesPerZone: 4
  minInstancesPerZone: 3 # $replicas -gt $ig_max_instances
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: cig5
spec:
  maxInstancesPerZone: 10
  minInstancesPerZone: 1 # $ig_min_instances <= $replicas <= $ig_max_instances
`
		stateMDs = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig1
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig1
spec:
  replicas: 1 # $ig_min_instances -ge $ig_max_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig20
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig20
spec: {} # "$replicas" == "null"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig21
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig21
spec:
  replicas: 0 # $replicas -eq 0
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig3
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig3
spec:
  replicas: 2 # $replicas -le $ig_min_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig4
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig4
spec:
  replicas: 7 # $replicas -gt $ig_max_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig5
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig5
spec:
  replicas: 5 # $ig_min_instances <= $replicas <= $ig_max_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-cig6
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: cig6 #cig6 is missing
spec:
  replicas: 5
`
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CloudInstanceGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with set of different pairs of MDs and CIGs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGs + stateMDs))
			f.RunHook()
		})

		It("", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig1").Field("spec.replicas").String()).To(Equal("2"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig20").Field("spec.replicas").String()).To(Equal("3"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig21").Field("spec.replicas").String()).To(Equal("3"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig3").Field("spec.replicas").String()).To(Equal("6"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig4").Field("spec.replicas").String()).To(Equal("4"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig5").Field("spec.replicas").String()).To(Equal("5"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-cig6").Field("spec.replicas").String()).To(Equal("5"))

			Expect(f.Session.Err).Should(gbytes.Say(`WARNING: can't find CloudInstanceGroup cig6 to get min and max instances per zone.`))
		})
	})
})
