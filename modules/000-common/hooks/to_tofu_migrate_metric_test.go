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

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: storage_classes :: to_tofu_migrate_metric", func() {

	stateTerraform := func() string {
		v := `
{
  "version": 4,
  "terraform_version": "0.14.8",
  "serial": 7,
  "lineage": "5d87d533-3c8a-0dc4-a02b-ed709674870b",
  "outputs": {},
  "resources": []
}
`
		return base64.StdEncoding.EncodeToString([]byte(v))
	}

	stateTofu := func() string {
		v := `
{"version":4,"terraform_version":"1.9.0","serial":8,"lineage":"5d87d533-3c8a-0dc4-a02b-ed709674870b","outputs":{},"resources":[]}
`
		return base64.StdEncoding.EncodeToString([]byte(v))
	}

	clusterState := func(state string, isBackup bool) string {
		bkpLabels := ""
		name := "d8-cluster-terraform-state"
		if isBackup {
			name = "tf-bkp-cluster-state"
			bkpLabels = "dhctl.deckhouse.io/before-tofu-state-backup: \"true\"\n    dhctl.deckhouse.io/state-backup: \"true\""
		}

		return fmt.Sprintf(`
apiVersion: v1
data:
  cluster-tf-state.json: %s
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    %s
  name: %s
  namespace: d8-system
type: Opaque
`, state, bkpLabels, name)
	}

	masterState := func(state string, isBackup bool) string {
		bkpLabels := ""
		name := "d8-node-terraform-state-nmit-delete-12-03-master-0"
		if isBackup {
			bkpLabels = "dhctl.deckhouse.io/before-tofu-state-backup: \"true\"\n    dhctl.deckhouse.io/state-backup: \"true\""
			name = "tf-bkp-node-nmit-delete-12-03-master-0"
		}
		return fmt.Sprintf(`
apiVersion: v1
data:
  node-tf-state.json: %s
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    node.deckhouse.io/node-group: master
    node.deckhouse.io/node-name: nmit-delete-12-03-master-0
    node.deckhouse.io/terraform-state: ""
    %s
  name: %s
  namespace: d8-system
type: Opaque
`, state, bkpLabels, name)
	}

	nodeState := func(state string, isBackup bool) string {
		bkpLabels := ""
		name := "d8-node-terraform-state-nmit-delete-12-03-khm-0"
		if isBackup {
			bkpLabels = "dhctl.deckhouse.io/before-tofu-state-backup: \"true\"\n    dhctl.deckhouse.io/state-backup: \"true\""
			name = "tf-bkp-node-nmit-delete-12-03-khm-0"
		}
		return fmt.Sprintf(`
apiVersion: v1
data:
  node-group-settings.json: c2VjcmV0
  node-tf-state.json: %s
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    node.deckhouse.io/node-group: khm
    node.deckhouse.io/node-name: nmit-delete-12-03-khm-0
    node.deckhouse.io/terraform-state: ""
    %s
  name: %s
  namespace: d8-system
type: Opaque`, state, bkpLabels, name)
	}

	const (
		initValuesString = `
global:
  discovery: {}
common: {}
`
	)

	f := HookExecutionConfigInit(initValuesString, `{}`)

	assertMetricVal := func(f *HookExecutionConfig, val float64) {
		Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(2))
		Expect(f.MetricsCollector.CollectedMetrics()[0].Group).To(Equal("D8MigrateToTofu"))
		Expect(f.MetricsCollector.CollectedMetrics()[0].Action).To(Equal(operation.ActionExpireMetrics))
		Expect(f.MetricsCollector.CollectedMetrics()[1].Name).To(Equal("d8_need_migrate_to_tofu"))
		Expect(f.MetricsCollector.CollectedMetrics()[1].Value).To(Equal(&val))
		Expect(f.MetricsCollector.CollectedMetrics()[1].Action).To(Equal(operation.ActionGaugeSet))
	}

	assertMetricSetToMigrate := func(f *HookExecutionConfig) {
		assertMetricVal(f, 1.0)
	}

	assertMetricSetToNotMigrate := func(f *HookExecutionConfig) {
		assertMetricVal(f, 0.0)
	}

	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should execute successfully (hybrid cluster case)", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has only terraform cluster state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterState(stateTerraform(), false)))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should set metric to migrate", func() {
			assertMetricSetToMigrate(f)
		})
	})

	Context("Cluster has terraform cluster state and master terraform state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusterState(stateTerraform(), false) + "\n---\n" + masterState(stateTerraform(), false)))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should set metric to migrate", func() {
			assertMetricSetToMigrate(f)
		})
	})

	Context("Cluster has terraform cluster state and master terraform state and node terraform state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(
				clusterState(stateTerraform(), false) +
					"\n---\n" +
					masterState(stateTerraform(), false) +
					"\n---\n" +
					nodeState(stateTerraform(), false),
			))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should set metric to migrate", func() {
			assertMetricSetToMigrate(f)
		})
	})

	Context("Partially migrate", func() {
		Context("Cluster has tofu cluster state and master terraform state and node terraform state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTerraform(), false) +
						"\n---\n" +
						nodeState(stateTerraform(), false),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to migrate", func() {
				assertMetricSetToMigrate(f)
			})
		})

		Context("Cluster has tofu cluster state and master tofu state and node terraform state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTofu(), false) +
						"\n---\n" +
						nodeState(stateTerraform(), false),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to migrate", func() {
				assertMetricSetToMigrate(f)
			})
		})

		Context("Cluster has tofu cluster state and master terraform state and node tofu state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTerraform(), false) +
						"\n---\n" +
						nodeState(stateTofu(), false),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to migrate", func() {
				assertMetricSetToMigrate(f)
			})
		})
	})

	Context("Fully migrate", func() {
		Context("Cluster has tofu cluster state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(clusterState(stateTofu(), false)))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to not migrate", func() {
				assertMetricSetToNotMigrate(f)
			})
		})

		Context("Cluster has tofu cluster state and master tofu state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTofu(), false),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to not migrate", func() {
				assertMetricSetToNotMigrate(f)
			})
		})

		Context("Cluster has tofu cluster state and master tofu state and node tofu state", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTofu(), false) +
						"\n---\n" +
						nodeState(stateTofu(), false),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to not migrate", func() {
				assertMetricSetToNotMigrate(f)
			})
		})
	})

	Context("Fully migrate and backups skip", func() {
		Context("Cluster has tofu cluster state and terraform cluster backup", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTerraform(), true) +
						"\n---\n" +
						clusterState(stateTofu(), false),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to not migrate", func() {
				assertMetricSetToNotMigrate(f)
			})
		})

		Context("Cluster has tofu cluster and terraform backup cluster state and master tofu state and mater terraform backup", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTerraform(), true) +
						"\n---\n" +
						clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTerraform(), true),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to not migrate", func() {
				assertMetricSetToNotMigrate(f)
			})
		})

		Context("Cluster has tofu cluster state and master tofu state and node tofu state and their backups", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					clusterState(stateTerraform(), true) +
						"\n---\n" +
						clusterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTofu(), false) +
						"\n---\n" +
						masterState(stateTerraform(), true) +
						"\n---\n" +
						nodeState(stateTofu(), false) +
						"\n---\n" +
						nodeState(stateTerraform(), true),
				))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set metric to not migrate", func() {
				assertMetricSetToNotMigrate(f)
			})
		})
	})
})
