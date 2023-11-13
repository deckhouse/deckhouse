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
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: deckhouse_admission_policy_engine_mark", func() {

	Context("Cluster with wrong semver", func() {
		f := HookExecutionConfigInit(`{"global": {"discovery": {}, "deckhouseVersion": "1.55.1"}}`, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8nsWithoutAnnotations))
			createNamespace(d8nsWithoutAnnotations)
			f.RunHook()
		})
		It("d8-namespace shouldn't have admission-policy-engine annotation set", func() {
			Expect(f).To(ExecuteSuccessfully())
			namespace := f.KubernetesGlobalResource("Namespace", d8Namespace)
			Expect(namespace.Field(fmt.Sprintf("metadata.annotations.%s", strings.ReplaceAll(admissionPolicyEngineAnnotation, ".", "\\."))).Exists()).ToNot(BeTrue())
		})
	})

	Context("Cluster with deckhouse v1.55.1 and annotation isn't set", func() {
		f := HookExecutionConfigInit(`{"global": {"discovery": {}, "deckhouseVersion": "v1.55.1"}}`, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8nsWithoutAnnotations))
			createNamespace(d8nsWithoutAnnotations)
			f.RunHook()
		})
		It("d8-namespace should have admission-policy-engine annotation set", func() {
			Expect(f).To(ExecuteSuccessfully())
			namespace := f.KubernetesGlobalResource("Namespace", d8Namespace)
			Expect(namespace.Field(fmt.Sprintf("metadata.annotations.%s", strings.ReplaceAll(admissionPolicyEngineAnnotation, ".", "\\."))).Exists()).To(BeTrue())
		})
	})

	Context("Cluster with deckhouse v1.55.3 and annotation is set", func() {
		f := HookExecutionConfigInit(`{"global": {"discovery": {}, "deckhouseVersion": "v1.55.3"}}`, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8nsWithAnnotations))
			createNamespace(d8nsWithAnnotations)
			f.RunHook()
		})
		It("d8-namespace should have admission-policy-engine annotation set", func() {
			Expect(f).To(ExecuteSuccessfully())
			namespace := f.KubernetesGlobalResource("Namespace", d8Namespace)
			fmt.Println(namespace)
			Expect(namespace.Field(fmt.Sprintf("metadata.annotations.%s", strings.ReplaceAll(admissionPolicyEngineAnnotation, ".", "\\."))).Exists()).To(BeTrue())
		})
	})

	Context("Cluster with deckhouse v1.56.1 and annotation isn't set", func() {
		f := HookExecutionConfigInit(`{"global": {"discovery": {}, "deckhouseVersion": "v1.56.1"}}`, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8nsWithoutAnnotations))
			createNamespace(d8nsWithoutAnnotations)
			f.RunHook()
		})
		It("d8-namespace shouldn't have admission-policy-engine annotation set", func() {
			Expect(f).To(ExecuteSuccessfully())
			namespace := f.KubernetesGlobalResource("Namespace", d8Namespace)
			Expect(namespace.Field(fmt.Sprintf("metadata.annotations.%s", strings.ReplaceAll(admissionPolicyEngineAnnotation, ".", "\\."))).Exists()).ToNot(BeTrue())
		})
	})
})

var (
	d8nsWithoutAnnotations = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
`
	d8nsWithAnnotations = `
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    admission-policy-engine.deckhouse.io/pss-profile-milestone: manually
  name: d8-system
`
)

func createNamespace(manifest string) {
	var ns corev1.Namespace
	_ = yaml.Unmarshal([]byte(manifest), &ns)
	_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().Namespaces().Create(context.TODO(), &ns, metav1.CreateOptions{})
}
