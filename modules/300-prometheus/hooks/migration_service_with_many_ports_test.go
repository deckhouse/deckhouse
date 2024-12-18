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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	serviceYAML = `
---
apiVersion: v1
kind: Service
metadata:
  name: memcached
  namespace: d8-monitoring
spec:
  ports:
    - name: memcached
      port: 11211
      targetPort: memcached
    - name: http-metrics
      port: 9150
      targetPort: http-metrics`
)

var _ = Describe("Prometheus hooks :: migration_service_with_many_ports", func() {
	f := HookExecutionConfigInit("{}", "{}")

	Context("There is a broken service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(serviceYAML))
			f.RunHook()
		})
		It("Service have been deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Service", "memcached").Exists()).To(BeFalse())
		})
	})
})
