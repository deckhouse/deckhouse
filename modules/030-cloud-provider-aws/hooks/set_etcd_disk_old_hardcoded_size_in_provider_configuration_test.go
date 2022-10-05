/*
Copyright 2022 Flant JSC

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

var _ = Describe("Modules :: cloud-provider-aws :: hooks :: set_etcd_disk_old_hardcoded_size_in_provider_configuration ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("With d8-provider-cluster-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxCmtpbmQ6IEFXU0NsdXN0ZXJDb25maWd1cmF0aW9uCm1hc3Rlck5vZGVHcm91cDoKICBpbnN0YW5jZUNsYXNzOgogICAgZmxhdm9yTmFtZTogdGVzdApzc2hQdWJsaWNLZXk6IHRlc3QK
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
`, 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field(`data.cloud-provider-cluster-configuration\.yaml`).String()).To(Equal("YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxCmtpbmQ6IEFXU0NsdXN0ZXJDb25maWd1cmF0aW9uCm1hc3Rlck5vZGVHcm91cDoKICBpbnN0YW5jZUNsYXNzOgogICAgZXRjZERpc2s6CiAgICAgIHNpemVHYjogMTUwCiAgICAgIHR5cGU6IGdwMgogICAgZmxhdm9yTmFtZTogdGVzdApzc2hQdWJsaWNLZXk6IHRlc3QK"))
		})

	})

})
