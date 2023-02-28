/*
Copyright 2023 Flant JSC

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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-aws :: hooks :: set_etcd_disk_old_hardcoded_size_in_provider_configuration ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	const clusterConfigurationBeforeMigration = `
masterNodeGroup:
  instanceClass:
    flavorName: test
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
sshPublicKey: test
`
	const clusterConfigurationAfterMigration = `
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
sshPublicKey: test
masterNodeGroup:
  instanceClass:
    etcdDisk:
      sizeGb: 150
      type: gp2
    flavorName: test
`
	secretBeforeMigration := fmt.Sprintf(`
---
apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: %s
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationBeforeMigration)))

	secretAfterMigration := fmt.Sprintf(`
---
apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: %s
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
`, base64.StdEncoding.EncodeToString([]byte(clusterConfigurationAfterMigration)))

	Context("With d8-provider-cluster-configuration secret: before migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretBeforeMigration, 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration")
			Expect(secret.Exists()).To(BeTrue())
			var b64 = base64.StdEncoding
			clusterConfuguration, _ := b64.DecodeString(secret.Field(`data.cloud-provider-cluster-configuration\.yaml`).String())
			Expect(string(clusterConfuguration)).To(MatchYAML(clusterConfigurationAfterMigration))
		})
	})

	Context("already migrated", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretAfterMigration, 1))
			f.RunHook()
		})

		It("Should not change the configuration", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration")
			Expect(secret.Exists()).To(BeTrue())
			var b64 = base64.StdEncoding
			clusterConfuguration, _ := b64.DecodeString(secret.Field(`data.cloud-provider-cluster-configuration\.yaml`).String())
			Expect(string(clusterConfuguration)).To(MatchYAML(clusterConfigurationAfterMigration))
		})
	})

})
