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
  name: deckhouse
  namespace: d8-system
  labels:
    migration.deckhouse.io/fix-services-broken-by-helm: done
spec:
  ports:
    - name: self
      port: 8080
      targetPort: self
      protocol: TCP
    - name: webhook
      port: 4223
      targetPort: webhook
      protocol: TCP`

	service2YAML = `
---
apiVersion: v1
kind: Service
metadata:
  name: deckhouse-leader
  namespace: d8-system
spec:
  ports:
    - name: self
      port: 8080
      targetPort: self
      protocol: TCP
    - name: webhook
      port: 4223
      targetPort: webhook
      protocol: TCP`
)

var _ = Describe("Deckhouse hooks :: migration_service_with_many_ports", func() {
	f := HookExecutionConfigInit("{}", "{}")

	Context("There is a broken service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(serviceYAML + service2YAML))
			f.RunHook()
		})
		It("Service have been deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Service", "deckhouse-leader").Exists()).To(BeFalse())
		})
	})
})
