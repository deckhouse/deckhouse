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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: loki :: hooks :: calculate_storage_capacity ::", func() {
	const (
		highLogsThroughputRate = 128
		pvcs                   = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: loki
  name: storage-loki-0
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 70Gi
  storageClassName: test
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: loki
  name: storage-loki-1
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 170Gi
  storageClassName: test
`
	)

	f := HookExecutionConfigInit(`{"loki": {"internal":{}}}`, `{"loki": {"diskSizeGigabytes": 50, "lokiConfig": {"ingestionRateMB": 4}}}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully; loki disk size must be 50 GiB, threshold: 46 GiB", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("loki.internal.pvcSize").Int()).To(Equal(int64(50 << 30)))
			Expect(f.ValuesGet("loki.internal.cleanupThreshold").Int()).To(Equal(int64(50 << 30 * 92 / 100)))
		})
	})

	Context("Cluster with PVCs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcs))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("must be executed successfully; loki disk size must be 70 GiB, retention must be 64.4 GiB", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("loki.internal.pvcSize").Int()).To(Equal(int64(70 << 30)))
			Expect(f.ValuesGet("loki.internal.cleanupThreshold").Int()).To(Equal(int64(70 << 30 * 92 / 100)))
		})
	})

	Context("Cluster with PVC and high logs throughput", func() {
		BeforeEach(func() {
			f.ValuesSet("loki.lokiConfig.ingestionRateMB", highLogsThroughputRate)
			f.BindingContexts.Set(f.KubeStateSet(pvcs))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("must be executed successfully; loki disk size must be 70, retention must be 55 GiB", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("loki.internal.pvcSize").Int()).To(Equal(int64(70 << 30)))
			Expect(f.ValuesGet("loki.internal.cleanupThreshold").Int()).To(Equal(int64(70<<30 - highLogsThroughputRate<<20*60*2)))
		})
	})
})
