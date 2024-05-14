// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: enable_cloud_provider ::", func() {
	secretManifest := func(content string) string {
		return `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(content))
	}
	clusterConfigManifest := func(provider string) string {
		data := `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: ` + provider + `
  prefix: kube
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.29"
`
		return secretManifest(data)
	}
	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Cluster has no d8-cluster-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should not enable any cloud providers", func() {
			Expect(f).To(ExecuteSuccessfully())

			for _, valuesName := range cloudProviderNameToModule {
				Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", valuesName)).Exists()).To(BeFalse())
			}
		})
	})

	for provider, valueName := range cloudProviderNameToModule {
		Context("Cluster has a d8-cluster-configuration secret with provider "+provider, func() {
			provider := provider

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(clusterConfigManifest(provider)))
				f.RunHook()
			})
			It("Should enable only one provider "+provider, func() {
				Expect(f).To(ExecuteSuccessfully())
				for key, valuesName := range cloudProviderNameToModule {
					if key == provider {
						Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", valuesName)).Bool()).To(BeTrue())
						continue
					}
					Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", valuesName)).Exists()).To(BeFalse())
				}
			})
		})

		Context("Cluster with "+valueName+" option in config", func() {
			provider := provider

			BeforeEach(func() {
				valueName := valueName

				f.BindingContexts.Set(f.KubeStateSet(``))
				f.ConfigValuesSetFromYaml(valueName, []byte("{}"))
				f.RunHook()
			})
			It("Should enable only one provider "+provider, func() {
				provider := provider

				Expect(f).To(ExecuteSuccessfully())
				for key, value := range cloudProviderNameToModule {
					if key == provider {
						Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", value)).Bool()).To(BeTrue())
						continue
					}
					Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", value)).Exists()).To(BeFalse())
				}
			})
		})
	}

	Context("Cluster has a d8-cluster-configuration secret for static cluster", func() {
		const clusterConf = `
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.244.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 192.168.0.0/16
kubernetesVersion: "1.29"
clusterDomain: cluster.local
defaultCRI: Containerd

`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretManifest(clusterConf)))
			f.RunHook()
		})
		It("Should not enable any provider", func() {
			Expect(f).To(ExecuteSuccessfully())

			for valueName := range cloudProviderNameToModule {
				Expect(f.ValuesGet(fmt.Sprintf("%sEnabled", valueName)).Exists()).To(BeFalse())

			}
		})
	})
})
