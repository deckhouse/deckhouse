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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: migrate/add_validation_label ::", func() {
	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	Context("Cluster with secret", func() {
		const (
			secretClusterConfiguration = `
---
apiVersion: v1
kind: Secret
metadata:
  labels:
    heritage: deckhouse
  name: d8-cluster-configuration
  namespace: kube-system
data:
  cluster-configuration.yaml: testdata
`
		)

		var secret *corev1.Secret
		_ = yaml.Unmarshal([]byte(secretClusterConfiguration), &secret)

		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretClusterConfiguration))
			_, _ = f.KubeClient().CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			f.RunHook()
		})

		It("Hook does not fail and add new label", func() {
			Expect(f).To(ExecuteSuccessfully())
			patchedSecret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			Expect(patchedSecret.Exists()).To(BeTrue())
			Expect(patchedSecret.Field("metadata.labels.name").String()).To(Equal("d8-cluster-configuration"))
		})
	})
})
