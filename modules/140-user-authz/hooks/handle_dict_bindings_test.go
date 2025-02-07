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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: user-authz :: hooks :: handle-dict-bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal": {}}}`, "")

	Context("There`s UseBinding", func() {
		BeforeEach(func() {
			resources := []string{
				useBinding("test-ns", rbacv1.Subject{Kind: "Group", Name: "testGroup"}),
				useBinding("test-ns2", rbacv1.Subject{Kind: "ServiceAccount", Namespace: "test-ns2", Name: "testSa"}),
				useBinding("test-ns3", rbacv1.Subject{Kind: "User", Namespace: "test-ns3", Name: "testUser"}),

				useBinding("ns2", rbacv1.Subject{Kind: "ServiceAccount", Namespace: "ns2", Name: "testsa"}),

				dictBinding("d8:dict:sa:ns:testsa", rbacv1.Subject{Kind: "ServiceAccount", Namespace: "ns", Name: "testsa"}),
				dictBinding("d8:dict:sa:ns2:testsa", rbacv1.Subject{Kind: "ServiceAccount", Namespace: "ns2", Name: "testsa"}),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should create DictClusterRoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("ClusterRoleBinding", "", "d8:dict:group:testgroup")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:dict:group:testgroup"))

			roleBinding = f.KubernetesResource("ClusterRoleBinding", "", "d8:dict:sa:test-ns2:testsa")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:dict:sa:test-ns2:testsa"))

			roleBinding = f.KubernetesResource("ClusterRoleBinding", "", "d8:dict:user:test-ns3:testuser")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:dict:user:test-ns3:testuser"))
		})

		It("Should delete DictClusterRoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("ClusterRoleBinding", "", "d8:dict:sa:ns:testsa")
			Expect(roleBinding).To(BeEmpty())
		})

		It("Should not delete DictRoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("ClusterRoleBinding", "", "d8:dict:sa:ns2:testsa")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:dict:sa:ns2:testsa"))
		})
	})
})

func useBinding(namespace string, subject rbacv1.Subject) string {
	binding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "d8:use:role:admin",
		},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}

func dictBinding(name string, subject rbacv1.Subject) string {
	binding := rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":                    "deckhouse",
				"rbac.deckhouse.io/automated": "true",
				"rbac.deckhouse.io/dict":      "true",
			},
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "d8:use:dict",
		},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}
