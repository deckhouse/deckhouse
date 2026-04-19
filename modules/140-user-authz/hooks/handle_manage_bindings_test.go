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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: user-authz :: hooks :: handle-manage-bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal": {}}}`, "")

	Context("There`s ManageSubsystemBinding", func() {
		BeforeEach(func() {
			resources := []string{
				manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
				manageModuleRole("d8:manage:permission:module:test2:edit", "others", "test2-ns"),
				manageRole("d8:manage:others:manager", "subsystem", "others"),
				manageBinding("test", "d8:manage:others:manager"),

				manageModuleRole("d8:manage:permission:module:test3:edit", "test", "test2-ns"),
				manageRole("d8:manage:test:manager", "subsystem", "test"),
				manageBinding("test2", "d8:manage:test:manager"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should create RoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:use:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:use:admin:binding:test"))
			roleBinding = f.KubernetesResource("RoleBinding", "test2-ns", "d8:use:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:use:admin:binding:test"))

			roleBinding = f.KubernetesResource("RoleBinding", "test2-ns", "d8:use:admin:binding:test2")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:use:admin:binding:test2"))
		})
	})

	Context("There`s ManageAllBinding", func() {
		BeforeEach(func() {
			resources := []string{
				manageModuleRole("d8:manage:permission:module:test:edit", "others", "test-ns"),
				manageModuleRole("d8:manage:permission:module:test2:edit", "others", "test2-ns"),
				manageRole("d8:manage:others:manager", "subsystem", "others"),
				manageRole("d8:manage:all:manager", "all", "all"),
				manageBinding("test", "d8:manage:all:manager"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should create RoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:use:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:use:admin:binding:test"))
			roleBinding = f.KubernetesResource("RoleBinding", "test2-ns", "d8:use:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:use:admin:binding:test"))
		})
	})

	Context("There`s UseBinding", func() {
		BeforeEach(func() {
			resources := []string{
				useAutomaticBinding("test", "test-ns"),
				useAutomaticBinding("test2", "test-ns"),
				useAutomaticBinding("test3", "test-ns2"),
				useAutomaticBinding("test4", "test-ns2"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should delete RoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:use:admin:binding:test")
			Expect(roleBinding).To(BeEmpty())
			roleBinding = f.KubernetesResource("RoleBinding", "test-ns", "d8:use:admin:binding:test2")
			Expect(roleBinding).To(BeEmpty())
			roleBinding = f.KubernetesResource("RoleBinding", "test-ns2", "d8:use:admin:binding:test3")
			Expect(roleBinding).To(BeEmpty())
			roleBinding = f.KubernetesResource("RoleBinding", "test-ns2", "d8:use:admin:binding:test4")
			Expect(roleBinding).To(BeEmpty())
		})
	})
})

func manageRole(name, level, subsystem string) string {
	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"rbac.deckhouse.io/use-role": "admin",
				"rbac.deckhouse.io/level":    level,
				"rbac.deckhouse.io/kind":     "manage",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					"rbac.deckhouse.io/kind": "manage",
					fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", subsystem): "manager",
				},
			},
		}},
	}
	if level != "all" {
		role.Labels["rbac.deckhouse.io/aggregate-to-all-as"] = "manager"
	}
	marshaled, _ := yaml.Marshal(&role)
	return string(marshaled)
}

func manageModuleRole(name, subsystem, namespace string) string {
	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"rbac.deckhouse.io/level":                                      "module",
				"rbac.deckhouse.io/kind":                                       "manage",
				"rbac.deckhouse.io/namespace":                                  namespace,
				fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", subsystem): "manager",
			},
		},
	}
	marshaled, _ := yaml.Marshal(&role)
	return string(marshaled)
}

func manageBinding(name, role string) string {
	binding := rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "test",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io/v1",
			Kind:     "ClusterRole",
			Name:     role,
		},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}

func useAutomaticBinding(relatedWith, namespace string) string {
	binding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("d8:binding:%s", relatedWith),
			Namespace: namespace,
			Labels: map[string]string{
				"heritage":                    "deckhouse",
				"rbac.deckhouse.io/automated": "true",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "test",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "d8:use:role",
		},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}
