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

package fencing

import (
	"encoding/base64"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const (
	testKey1 = "dh-test-secret-0123456789abcdef0"
	testKey2 = "dh-test-secret-0123456789abcdef1"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func keySecretYAML(annotations string, data map[string]string) string {
	out := fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: fencing-memberlist-key
  namespace: d8-cloud-instance-manager
%s
data:
`, annotations)
	for name, value := range data {
		out += fmt.Sprintf("  %s: %s\n", name, b64(value))
	}
	return out
}

const agentPodV2YAML = `
---
apiVersion: v1
kind: Pod
metadata:
  name: fencing-agent-worker-vwxyz
  namespace: d8-cloud-instance-manager
  labels:
    app: fencing-agent
    node-manager.deckhouse.io/fencing-agent-version: "2"
`

var _ = Describe("Modules :: node-manager :: hooks :: fencing :: memberlist_key ::", func() {
	const fencingOn = `{"workers":{"mode":"Slow"},"other":{"mode":"Disabled"}}`

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"fencing":{}}}}`, `{}`)

	expire := operation.MetricOperation{Group: keyRegeneratedMetric, Action: operation.ActionExpireMetrics}
	timestamp := func(t time.Time) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   keyRegeneratedMetric,
			Group:  keyRegeneratedMetric,
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(float64(t.Unix())),
		}
	}
	secret := func() object_store.KubeObject {
		return f.KubernetesResource("Secret", "d8-cloud-instance-manager", "fencing-memberlist-key")
	}
	now := func() time.Time { return dependency.TestDC.GetClock().Now() }

	BeforeEach(func() {
		f.ValuesSetFromYaml("nodeManager.internal.fencing.nodeGroups", []byte(fencingOn))
	})

	Context("First install: no Secret and no agent pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunGoHook()
		})

		It("Must create the Secret with one new key, without regeneration time and metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(secret().Exists()).To(BeTrue())
			Expect(secret().Field("data").Map()).To(HaveLen(1))
			key, err := base64.StdEncoding.DecodeString(secret().Field("data.primary").String())
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(HaveLen(32))
			Expect(secret().Field("metadata.annotations").Exists()).To(BeFalse())
			Expect(secret().Field("metadata.labels.heritage").Exists()).To(BeFalse())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})

		It("Must not put the key into values", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("nodeManager.internal.fencing").Map()).To(HaveLen(1))
			Expect(f.ValuesGet("nodeManager.internal.fencing.nodeGroups").Exists()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager").String()).NotTo(ContainSubstring("primary"))
		})
	})

	Context("The Secret was deleted while agents of version 2 work", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(agentPodV2YAML))
			f.RunGoHook()
		})

		It("Must create a new key, write the regeneration time and publish the metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(secret().Field("data").Map()).To(HaveLen(1))
			Expect(secret().Field("metadata.annotations").Map()).To(HaveLen(1))
			Expect(secret().Field(`metadata.annotations.node-manager\.deckhouse\.io/fencing-key-regenerated-at`).String()).
				To(Equal(now().UTC().Format(time.RFC3339)))
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire, timestamp(now())}))
		})
	})

	Context("All groups are Disabled and there is no Secret", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.fencing.nodeGroups", []byte(`{"workers":{"mode":"Disabled"}}`))
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunGoHook()
		})

		It("Must not create the Secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(secret().Exists()).To(BeFalse())
		})
	})

	Context("An administrator wrote a new key first and kept the old key second", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(keySecretYAML("", map[string]string{"primary": testKey2, "secondary": testKey1}) + agentPodV2YAML))
			f.RunGoHook()
		})

		It("Must leave the Secret as it is and light no metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(secret().Field("data.primary").String()).To(Equal(b64(testKey2)))
			Expect(secret().Field("data.secondary").String()).To(Equal(b64(testKey1)))
			Expect(secret().Field("data").Map()).To(HaveLen(2))
			Expect(secret().Field("metadata.annotations").Exists()).To(BeFalse())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})
	})

	Context("The Secret has the regeneration time", func() {
		const regeneratedAt = "2026-10-20T10:00:00Z"
		annotations := "  annotations:\n    node-manager.deckhouse.io/fencing-key-regenerated-at: \"" + regeneratedAt + "\""

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(keySecretYAML(annotations, map[string]string{"primary": testKey1})))
		})

		It("Must publish the time from the annotation on every run, also after a restart", func() {
			want := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)

			for range 2 {
				f.RunGoHook()
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire, timestamp(want)}))
				Expect(secret().Field(`metadata.annotations.node-manager\.deckhouse\.io/fencing-key-regenerated-at`).String()).To(Equal(regeneratedAt))
			}
		})
	})

	Context("An administrator wrote a key of a wrong format", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(keySecretYAML("", map[string]string{"primary": "short"}) + agentPodV2YAML))
			f.RunGoHook()
		})

		It("Must not replace the key and must not light the metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(secret().Field("data.primary").String()).To(Equal(b64("short")))
			Expect(secret().Field("data").Map()).To(HaveLen(1))
			Expect(secret().Field("metadata.annotations").Exists()).To(BeFalse())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{expire}))
		})
	})
})
