// Copyright 2022 Flant JSC
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

// TODO remove after 1.42 release

package hooks

import (
	"context"
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	clusterConfiguration = `
apiVersion: deckhouse.io/v1
cloud:
  prefix: dev
  provider: OpenStack
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.23"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`

	proxyConfigurationWithoutAuth = `
packagesProxy:
  uri: http://1.2.3.4:80
`
	proxyConfigurationWithAuth = `
packagesProxy:
  uri: http://1.2.3.4:80
  username: test
  password: test
`
	proxyConfigurationWithWrongURI = `
packagesProxy:
  uri: http:/1.2.3.4:80
`
	proxyConfigurationMigrated = `
proxy:
  httpProxy: http://1.2.3.4
  httpsProxy: https://1.2.3.4
`
)

func createClusterConfigurationSecret(proxyConfiguration string) error {
	var secretTemplate = `
---
apiVersion: v1
data:
  %s: %s
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    name: d8-cluster-configuration
  name: d8-cluster-configuration
  namespace: kube-system
type: Opaque
`
	secret := fmt.Sprintf(secretTemplate, DataKey, base64.StdEncoding.EncodeToString([]byte(clusterConfiguration+proxyConfiguration)))
	var s v1.Secret
	err := yaml.Unmarshal([]byte(secret), &s)
	if err != nil {
		return err
	}
	_, err = dependency.TestDC.MustGetK8sClient().
		CoreV1().
		Secrets("kube-system").
		Create(context.TODO(), &s, metav1.CreateOptions{})
	return err
}

var _ = Describe("Global :: migrate_cluster_configuration ::", func() {
	const (
		initValuesString                = `{}`
		initConfigValuesString          = `{}`
		initConfigValuesWithProxyString = `{
  "global": {
    "modules": {
      "proxy": {
        "httpProxy": "http://1.2.3.4",
        "httpsProxy": "https://1.2.3.4"
      }
    }
  }
}`
	)

	Context("Secret kube-system/d8-cluster-configuration does not exist", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" contains proxy section, migration already done", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret(proxyConfigurationMigrated)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration + proxyConfigurationMigrated))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("proxy parameter is set, migration is not needed"))

		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" does not contain packagesProxy section, global.modules.proxy is not set, migration is not needed", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret("")
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("migration is not needed"))

		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" contains packagesProxy section without auth, global.modules.proxy is not set", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret(proxyConfigurationWithoutAuth)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration + `
proxy:
  httpProxy: http://1.2.3.4:80
  httpsProxy: http://1.2.3.4:80
`))
		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" contains packagesProxy section with auth, global.modules.proxy is not set", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret(proxyConfigurationWithAuth)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration + `
proxy:
  httpProxy: http://test:test@1.2.3.4:80
  httpsProxy: http://test:test@1.2.3.4:80
`))
		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" contains packagesProxy section with wrong URI, global.modules.proxy is not set", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret(proxyConfigurationWithWrongURI)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

		It("Hook does not change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration + proxyConfigurationWithWrongURI))
		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" does not contain packagesProxy section, global.modules.proxy is set", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesWithProxyString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret("")
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration + `
proxy:
  httpProxy: http://1.2.3.4
  httpsProxy: https://1.2.3.4
`))
		})
	})

	Context("Secret kube-system/d8-cluster-configuration exists, field "+DataKey+" contains packagesProxy section, global.modules.proxy is set", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesWithProxyString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createClusterConfigurationSecret(proxyConfigurationWithAuth)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not change secret", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(clusterConfiguration + `
proxy:
  httpProxy: http://1.2.3.4
  httpsProxy: https://1.2.3.4
`))
		})
	})

})
