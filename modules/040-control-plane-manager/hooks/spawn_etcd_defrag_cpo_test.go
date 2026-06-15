/*
Copyright 2026 Flant JSC

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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var cpoGVR = schema.GroupVersionResource{
	Group:    "control-plane.deckhouse.io",
	Version:  "v1alpha1",
	Resource: "controlplaneoperations",
}

// fixedNow is the injected "current time" for all spawn_etcd_defrag_cpo tests.
var defragTestNow = time.Date(2024, 6, 15, 10, 33, 0, 0, time.UTC)

// pastSlot is three minutes before fixedNow; Next("* * * * *", pastSlot) = 10:31 <= 10:33 → fires.
const defragPastSlot = "2024-06-15T10:30:00Z"

// currentSlotStr matches fixedNow truncated to minute; Next(..., currentSlot) = 10:34 > 10:33 → no fire.
const defragCurrentSlot = "2024-06-15T10:33:00Z"

const (
	valuesDefragEnabled = `{
		"global": {"clusterIsBootstrapped": true},
		"controlPlaneManager": {
			"internal": {
				"etcdDefrag": {"enabled": true, "cronSchedule": "* * * * *"}
			},
			"apiserver": {"authn": {}, "authz": {}}
		}
	}`

	valuesDefragDisabled = `{
		"global": {"clusterIsBootstrapped": true},
		"controlPlaneManager": {
			"internal": {
				"etcdDefrag": {"enabled": false, "cronSchedule": "* * * * *"}
			},
			"apiserver": {"authn": {}, "authz": {}}
		}
	}`

	valuesNotBootstrapped = `{
		"global": {"clusterIsBootstrapped": false},
		"controlPlaneManager": {
			"internal": {
				"etcdDefrag": {"enabled": true, "cronSchedule": "* * * * *"}
			},
			"apiserver": {"authn": {}, "authz": {}}
		}
	}`
)

func defragStateCMYAML(slot string) string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-control-plane-manager-etcd-defrag
  namespace: kube-system
data:
  lastHandledCronSlot: "%s"
`, slot)
}

const (
	defragMaster0 = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""`

	defragMaster1 = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""`

	defragMaster2 = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-2
  labels:
    node-role.kubernetes.io/control-plane: ""`

	defragArbiter = `
---
apiVersion: v1
kind: Node
metadata:
  name: arbiter-0
  labels:
    node.deckhouse.io/etcd-arbiter: ""`
)

func newDefragHook(values string) *HookExecutionConfig {
	f := HookExecutionConfigInit(values, "")
	f.RegisterCRD("control-plane.deckhouse.io", "v1alpha1", "ControlPlaneOperation", true)
	return f
}

func listCPOs(f *HookExecutionConfig) (int, []map[string]interface{}) {
	list, err := f.KubeClient().Dynamic().Resource(cpoGVR).Namespace("kube-system").List(context.TODO(), metav1.ListOptions{})
	Expect(err).ToNot(HaveOccurred())
	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, item.Object)
	}
	return len(list.Items), items
}

var _ = Describe("Modules :: control-plane-manager :: hooks :: spawn_etcd_defrag_cpo ::", func() {
	BeforeEach(func() {
		defragNow = func() time.Time { return defragTestNow }
	})
	AfterEach(func() {
		defragNow = func() time.Time { return time.Now().UTC() }
	})

	Context("defrag disabled in internal values", func() {
		f := newDefragHook(valuesDefragDisabled)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defragMaster0 + defragMaster1 + defragMaster2))
			f.RunHook()
		})
		It("executes successfully and creates no CPOs", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
		})
	})

	Context("cluster not bootstrapped", func() {
		f := newDefragHook(valuesNotBootstrapped)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defragMaster0 + defragMaster1 + defragMaster2))
			f.RunHook()
		})
		It("executes successfully and creates no CPOs", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
		})
	})

	Context("cron slot already handled (idempotency)", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			state := defragMaster0 + defragMaster1 + defragMaster2 + defragStateCMYAML(defragCurrentSlot)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})
		It("executes successfully and creates no CPOs", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
		})
	})

	Context("no etcd nodes", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			// ConfigMap with past slot so the cron fires, but there are no nodes.
			f.BindingContexts.Set(f.KubeStateSet(defragStateCMYAML(defragPastSlot)))
			f.RunHook()
		})
		It("executes successfully and creates no CPOs", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
		})
	})

	Context("cron fired, 3 master nodes, no ConfigMap", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defragMaster0 + defragMaster1 + defragMaster2))
			f.RunHook()
		})
		It("creates one CPO per master and updates the ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())

			count, cpos := listCPOs(f)
			Expect(count).To(Equal(3))

			for _, cpo := range cpos {
				spec, _ := cpo["spec"].(map[string]interface{})
				Expect(spec["component"]).To(Equal("Etcd"))
				Expect(spec["approved"]).To(Equal(false))
				steps, _ := spec["steps"].([]interface{})
				Expect(steps).To(ConsistOf("DefragEtcd"))

				labels, _ := cpo["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
				Expect(labels["control-plane.deckhouse.io/component"]).To(Equal("etcd"))
				Expect(labels["heritage"]).To(Equal("deckhouse"))
			}

			cm := f.KubernetesResource("ConfigMap", "kube-system", defragStateCMName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.lastHandledCronSlot").String()).To(Equal(defragTestNow.Truncate(time.Minute).Format(time.RFC3339)))
		})
	})

	Context("cron fired, 2 masters + 1 arbiter node", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			state := defragMaster0 + defragMaster1 + defragArbiter + defragStateCMYAML(defragPastSlot)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})
		It("creates 3 CPOs (masters + arbiter, deduplicated)", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(3))
		})
	})

	Context("cron fired, master also has arbiter label (edge case dedup)", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			// master-0 appears only in master snapshot; arbiter-0 is a separate node.
			// Both master-0 and arbiter-0 should produce exactly 2 CPOs, not 3.
			state := defragMaster0 + defragArbiter + defragStateCMYAML(defragPastSlot)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})
		It("creates exactly 2 CPOs without duplicates", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(2))
		})
	})
})
