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

	// Dict bindings are created with GenerateName, so they have no predictable
	// name to query in the fake cluster. We therefore assert on the patch
	// operations the hook produced (create/delete intent), and look up
	// fixed-name objects for deletions.
	createDescriptions := func() []string {
		var out []string
		for _, op := range f.PatchCollector.Operations() {
			if strings.HasPrefix(op.Description(), "Create object") {
				out = append(out, op.Description())
			}
		}
		return out
	}
	deleteDescriptions := func() []string {
		var out []string
		for _, op := range f.PatchCollector.Operations() {
			if strings.HasPrefix(op.Description(), "Delete object") {
				out = append(out, op.Description())
			}
		}
		return out
	}

	Context("A use binding exists for a subject without a dict binding", func() {
		BeforeEach(func() {
			resources := []string{
				useDictRoleBinding("alice-use", "alice-ns", "d8:namespace:user", userSubject("alice")),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("creates exactly one dict ClusterRoleBinding and deletes nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			creates := createDescriptions()
			Expect(creates).To(HaveLen(1))
			Expect(creates[0]).To(ContainSubstring("ClusterRoleBinding"))
			Expect(deleteDescriptions()).To(BeEmpty())
		})
	})

	Context("A legacy dict binding (d8:use:dict) exists and the subject still has a use binding", func() {
		BeforeEach(func() {
			resources := []string{
				// Legacy binding referencing the renamed-away role name.
				dictClusterRoleBinding("legacy-alice", "d8:use:dict", userSubject("alice")),
				useDictRoleBinding("alice-use", "alice-ns", "d8:namespace:user", userSubject("alice")),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("deletes the legacy binding and recreates it under d8:dict", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Legacy (d8:use:dict) binding is removed...
			Expect(deleteDescriptions()).To(ConsistOf(ContainSubstring("legacy-alice")))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "legacy-alice").Exists()).To(BeFalse())

			// ...and a fresh d8:dict binding is created for the still-present subject.
			Expect(createDescriptions()).To(HaveLen(1))
		})
	})

	Context("A legacy dict binding (d8:use:dict) exists with no matching use binding", func() {
		BeforeEach(func() {
			resources := []string{
				dictClusterRoleBinding("legacy-bob", "d8:use:dict", userSubject("bob")),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("deletes the legacy binding and creates nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(deleteDescriptions()).To(ConsistOf(ContainSubstring("legacy-bob")))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "legacy-bob").Exists()).To(BeFalse())
			Expect(createDescriptions()).To(BeEmpty())
		})
	})

	Context("A valid dict binding is kept while an orphaned one is deleted", func() {
		BeforeEach(func() {
			resources := []string{
				// alice still has a use binding -> her dict binding must survive.
				dictClusterRoleBinding("dict-alice", "d8:dict", userSubject("alice")),
				useDictRoleBinding("alice-use", "alice-ns", "d8:namespace:user", userSubject("alice")),
				// carol has no use binding -> her dict binding is an orphan.
				dictClusterRoleBinding("dict-carol", "d8:dict", userSubject("carol")),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("keeps alice and deletes only carol, without recreating alice", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(deleteDescriptions()).To(ConsistOf(ContainSubstring("dict-carol")))
			Expect(createDescriptions()).To(BeEmpty())

			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "dict-alice").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "dict-carol").Exists()).To(BeFalse())
		})
	})

	Context("The same subject is granted by several use bindings", func() {
		BeforeEach(func() {
			resources := []string{
				useDictRoleBinding("dave-use-1", "dave-ns", "d8:namespace:user", userSubject("dave")),
				useDictRoleBinding("dave-use-2", "other-ns", "d8:namespace:admin", userSubject("dave")),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("deduplicates to a single dict binding", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createDescriptions()).To(HaveLen(1))
		})
	})

	Context("Two distinct subjects share a long (>55 char) key prefix", func() {
		// The old dedup key truncated to 55 characters, so two subjects sharing
		// a 55-char prefix collided and one dict binding was lost. The full-key
		// fix must keep them distinct (two creates rather than one).
		longNamespace := strings.Repeat("n", 60)

		BeforeEach(func() {
			resources := []string{
				useDictRoleBinding("sa-use-1", "app-ns", "d8:namespace:user", saSubject(longNamespace, "one")),
				useDictRoleBinding("sa-use-2", "app-ns", "d8:namespace:user", saSubject(longNamespace, "two")),
			}
			f.KubeStateSet(strings.Join(resources, "\n---\n"))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("creates a distinct dict binding for each subject", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(createDescriptions()).To(HaveLen(2))
		})
	})
})

func dictClusterRoleBinding(name, roleName string, subject rbacv1.Subject) string {
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
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{subject},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}

// useDictRoleBinding builds a use RoleBinding that filterUseBinding accepts via
// isD8DictBinding: it references a d8:namespace:* role and carries no
// heritage=deckhouse label.
func useDictRoleBinding(name, namespace, roleName string, subject rbacv1.Subject) string {
	binding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{subject},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}

func userSubject(name string) rbacv1.Subject {
	return rbacv1.Subject{
		Kind:     "User",
		APIGroup: "rbac.authorization.k8s.io",
		Name:     name,
	}
}

func saSubject(namespace, name string) rbacv1.Subject {
	return rbacv1.Subject{
		Kind:      "ServiceAccount",
		Namespace: namespace,
		Name:      name,
	}
}
