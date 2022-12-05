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

// TODO remove after 1.38 release

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
	providerClusterConfigurationWithoutStandard = `
apiVersion: deckhouse.io/v1alpha1
kind: AWSClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    ami: ami-0943382e114f188e8
    instanceType: m5.xlarge
  replicas: 3
nodeNetworkCIDR: 10.240.0.0/20
provider:
  providerAccessKeyId: XXXXXXXXXXXXXX
  providerSecretAccessKey: XXXXXXXXXXXXXX
  region: eu-west-1
sshPublicKey: ssh-rsa XXXXXXXXXXXXXX
tags:
  Usage: test
vpcNetworkCIDR: 10.240.0.0/16
`

	providerClusterConfigurationWithStandard = `
apiVersion: deckhouse.io/v1alpha1
kind: AWSClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    ami: ami-0943382e114f188e8
    instanceType: m5.xlarge
  replicas: 3
nodeNetworkCIDR: 10.240.0.0/20
provider:
  providerAccessKeyId: XXXXXXXXXXXXXX
  providerSecretAccessKey: XXXXXXXXXXXXXX
  region: eu-west-1
standard:
  associatePublicIPToMasters: true
  associatePublicIPToNodes: true
sshPublicKey: ssh-rsa XXXXXXXXXXXXXX
tags:
  Usage: test
vpcNetworkCIDR: 10.240.0.0/16
`
)

func createProviderClusterConfigurationSecret(providerClusterConfiguration string) error {
	var secretTemplate = `
---
apiVersion: v1
data:
  %s: %s
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    name: d8-provider-cluster-configuration
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
`
	secret := fmt.Sprintf(secretTemplate, DataKey, base64.StdEncoding.EncodeToString([]byte(providerClusterConfiguration)))
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

var _ = Describe("Cloud-provider-aws :: migrate_aws_cluster_configuration ::", func() {
	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	Context("No config", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

		It("Hook does not generate secret", func() {
			_, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-provider-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(Not(BeNil()))
		})
	})

	Context("Config exists, field "+DataKey+" does not contain standard section", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createProviderClusterConfigurationSecret(providerClusterConfigurationWithoutStandard)
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
				Get(context.TODO(), "d8-provider-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(providerClusterConfigurationWithoutStandard))
		})
	})

	Context("Config exists, field "+DataKey+" contains standard section", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {

			f.KubeStateSet("")

			err := createProviderClusterConfigurationSecret(providerClusterConfigurationWithStandard)
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook changes secret, standard section is removed", func() {
			s, err := dependency.TestDC.K8sClient.CoreV1().
				Secrets("kube-system").
				Get(context.TODO(), "d8-provider-cluster-configuration", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(s.Data[DataKey]).To(MatchYAML(providerClusterConfigurationWithoutStandard))
		})
	})

})
