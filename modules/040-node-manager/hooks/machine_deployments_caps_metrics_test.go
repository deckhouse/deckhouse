/*
Copyright 2024 Flant JSC

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

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const MachineDeploymentWith1Unavailable = `
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: MachineDeployment
metadata:
  name: caps-worker
  namespace: d8-cloud-instance-manager
  labels:
    app.kubernetes.io/managed-by: Helm
    app: caps-controller
    cluster.x-k8s.io/cluster-name: static
    heritage: deckhouse
    module: node-manager
    node-group: caps-worker
spec:
  clusterName: static
  minReadySeconds: 0
  progressDeadlineSeconds: 600
  replicas: 2
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: static
      cluster.x-k8s.io/deployment-name: caps-worker
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
status:
  availableReplicas: 1
  observedGeneration: 9
  phase: ScalingUp
  readyReplicas: 1
  replicas: 2
  unavailableReplicas: 1
  updatedReplicas: 2
`

const MachineDeploymentWith1Available = `
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: MachineDeployment
metadata:
  name: caps-worker
  namespace: d8-cloud-instance-manager
  labels:
    app.kubernetes.io/managed-by: Helm
    app: caps-controller
    cluster.x-k8s.io/cluster-name: static
    heritage: deckhouse
    module: node-manager
    node-group: caps-worker
spec:
  clusterName: static
  minReadySeconds: 0
  progressDeadlineSeconds: 600
  replicas: 1
status:
  availableReplicas: 1
  observedGeneration: 9
  phase: Running
  readyReplicas: 1
  replicas: 1
  unavailableReplicas: 0
  updatedReplicas: 1
`

var _ = Describe("Modules :: node-manager :: hooks :: machine_deployments_caps_metrics_test ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachineDeployment", true)

	assertMetric := func(f *HookExecutionConfig, name string, expected float64) {
		metrics := f.MetricsCollector.CollectedMetrics()

		ok := false
		for _, m := range metrics {
			if m.Name == name {
				Expect(m.Value).To(Equal(ptr.To(expected)))
				Expect(m.Labels).To(HaveKey("machine_deployment_name"))
				Expect(m.Labels["machine_deployment_name"]).To(Equal("caps-worker"))

				ok = true

				break
			}
		}

		Expect(ok).To(BeTrue())
	}

	Context("MachineDeployment with 1 unavailable instance", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(MachineDeploymentWith1Unavailable))

			f.RunHook()
		})

		It("Test metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			tests := []struct {
				name  string
				value float64
			}{
				{
					name:  capsMachineDeploymentMetricReplicasName,
					value: 2.0,
				},
				{
					name:  capsMachineDeploymentMetricDesiredName,
					value: 2.0,
				},
				{
					name:  capsMachineDeploymentMetricReadyName,
					value: 1.0,
				},
				{
					name:  capsMachineDeploymentMetricUnavailableName,
					value: 1.0,
				},
				{
					name:  capsMachineDeploymentMetricPhaseName,
					value: 2.0,
				},
			}

			for _, test := range tests {
				assertMetric(f, test.name, test.value)
			}
		})
	})
})
