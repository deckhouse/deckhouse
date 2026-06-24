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
// 01:02 UTC — 2 minutes past the "0 1 * * *" cron slot, within the 5-min grace window.
var defragTestNow = time.Date(2024, 6, 15, 1, 2, 0, 0, time.UTC)

// defragPastSlot is before 01:00 so sched.Next(pastSlot) = 01:00 ≤ 01:02 → hook fires.
const defragPastSlot = "2024-06-15T00:59:00Z"

// defragCurrentSlot matches defragTestNow truncated to minute;
// sched.Next(currentSlot) lands on 2024-06-16T01:00 > 01:02 → no fire (idempotency).
const defragCurrentSlot = "2024-06-15T01:02:00Z"

const (
	valuesDefragEnabled = `{
		"global": {"clusterIsBootstrapped": true},
		"controlPlaneManager": {
			"internal": {
				"etcdDefrag": {"enabled": true, "cronSchedule": "0 1 * * *"}
			},
			"apiserver": {"authn": {}, "authz": {}}
		}
	}`

	valuesDefragDisabled = `{
		"global": {"clusterIsBootstrapped": true},
		"controlPlaneManager": {
			"internal": {
				"etcdDefrag": {"enabled": false, "cronSchedule": "0 1 * * *"}
			},
			"apiserver": {"authn": {}, "authz": {}}
		}
	}`

	valuesNotBootstrapped = `{
		"global": {"clusterIsBootstrapped": false},
		"controlPlaneManager": {
			"internal": {
				"etcdDefrag": {"enabled": true, "cronSchedule": "0 1 * * *"}
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
	defragCPN0 = `
---
apiVersion: control-plane.deckhouse.io/v1alpha1
kind: ControlPlaneNode
metadata:
  name: master-0
  namespace: kube-system
  uid: "uid-master-0"`

	defragCPN1 = `
---
apiVersion: control-plane.deckhouse.io/v1alpha1
kind: ControlPlaneNode
metadata:
  name: master-1
  namespace: kube-system
  uid: "uid-master-1"`

	defragCPN2 = `
---
apiVersion: control-plane.deckhouse.io/v1alpha1
kind: ControlPlaneNode
metadata:
  name: master-2
  namespace: kube-system
  uid: "uid-master-2"`

	// defragCPNArbiter represents an etcd-arbiter node — the configuration controller
	// creates ControlPlaneNode objects for both master and arbiter nodes.
	defragCPNArbiter = `
---
apiVersion: control-plane.deckhouse.io/v1alpha1
kind: ControlPlaneNode
metadata:
  name: arbiter-0
  namespace: kube-system
  uid: "uid-arbiter-0"`
)

func newDefragHook(values string) *HookExecutionConfig {
	f := HookExecutionConfigInit(values, "")
	f.RegisterCRD("control-plane.deckhouse.io", "v1alpha1", "ControlPlaneOperation", true)
	f.RegisterCRD("control-plane.deckhouse.io", "v1alpha1", "ControlPlaneNode", true)
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
		defragNow = time.Now
	})

	Context("defrag disabled in internal values", func() {
		f := newDefragHook(valuesDefragDisabled)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defragCPN0 + defragCPN1 + defragCPN2))
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
			f.BindingContexts.Set(f.KubeStateSet(defragCPN0 + defragCPN1 + defragCPN2))
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
			state := defragCPN0 + defragCPN1 + defragCPN2 + defragStateCMYAML(defragCurrentSlot)
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
			// ConfigMap with past slot so the cron fires, but there are no CPNs.
			f.BindingContexts.Set(f.KubeStateSet(defragStateCMYAML(defragPastSlot)))
			f.RunHook()
		})
		It("executes successfully and creates no CPOs", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
		})
	})

	Context("first install: no ConfigMap", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defragCPN0 + defragCPN1 + defragCPN2))
			f.RunHook()
		})
		It("initializes the ConfigMap without creating CPOs", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
			cm := f.KubernetesResource("ConfigMap", "kube-system", defragStateCMName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.lastHandledCronSlot").String()).To(Equal(defragTestNow.Truncate(time.Minute).Format(time.RFC3339)))
		})
	})

	Context("cron fired, 3 master nodes, past slot in ConfigMap", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			state := defragCPN0 + defragCPN1 + defragCPN2 + defragStateCMYAML(defragPastSlot)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})
		It("creates one CPO per master with ownerReference and updates the ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())

			count, cpos := listCPOs(f)
			Expect(count).To(Equal(3))

			for _, cpo := range cpos {
				spec, _ := cpo["spec"].(map[string]interface{})
				Expect(spec["component"]).To(Equal("Etcd"))
				Expect(spec["approved"]).To(Equal(false))
				steps, _ := spec["steps"].([]interface{})
				Expect(steps).To(ConsistOf("DefragEtcd", "WaitPodReady"))

				meta, _ := cpo["metadata"].(map[string]interface{})
				labels, _ := meta["labels"].(map[string]interface{})
				Expect(labels["control-plane.deckhouse.io/component"]).To(Equal("etcd"))
				Expect(labels["heritage"]).To(Equal("deckhouse"))
				Expect(labels["control-plane.deckhouse.io/slot"]).NotTo(BeEmpty())

				ownerRefs, _ := meta["ownerReferences"].([]interface{})
				Expect(ownerRefs).To(HaveLen(1))
				ref, _ := ownerRefs[0].(map[string]interface{})
				Expect(ref["kind"]).To(Equal("ControlPlaneNode"))
				Expect(ref["apiVersion"]).To(Equal("control-plane.deckhouse.io/v1alpha1"))
				Expect(ref["controller"]).To(Equal(true))
			}

			cm := f.KubernetesResource("ConfigMap", "kube-system", defragStateCMName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.lastHandledCronSlot").String()).To(Equal(defragTestNow.Truncate(time.Minute).Format(time.RFC3339)))
		})
	})

	Context("cron fired, 2 masters + 1 arbiter node", func() {
		f := newDefragHook(valuesDefragEnabled)
		BeforeEach(func() {
			state := defragCPN0 + defragCPN1 + defragCPNArbiter + defragStateCMYAML(defragPastSlot)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})
		It("creates 3 CPOs (masters + arbiter)", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(3))
		})
	})

	Context("grace period exceeded: slot missed by more than 5 minutes", func() {
		f := newDefragHook(valuesDefragEnabled)
		// missedSlot from the previous day: sched.Next(missedSlot) = 2024-06-14T01:00,
		// delay from currentSlot (2024-06-15T01:02) = ~24h >> 5-min grace period.
		const missedSlot = "2024-06-14T00:00:00Z"
		BeforeEach(func() {
			state := defragCPN0 + defragCPN1 + defragCPN2 + defragStateCMYAML(missedSlot)
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})
		It("creates no CPOs and sets lastHandledCronSlot to currentSlot to resume on the next tick", func() {
			Expect(f).To(ExecuteSuccessfully())
			count, _ := listCPOs(f)
			Expect(count).To(Equal(0))
			cm := f.KubernetesResource("ConfigMap", "kube-system", defragStateCMName)
			Expect(cm.Exists()).To(BeTrue())
			// Must jump to currentSlot (not nextSlot) so the schedule resumes immediately
			// rather than slowly advancing one slot per tick through the entire missed gap.
			slot := cm.Field("data.lastHandledCronSlot").String()
			Expect(slot).To(Equal(defragTestNow.Truncate(time.Minute).Format(time.RFC3339)))
		})
	})
})
