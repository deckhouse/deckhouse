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

	Context("There`s SubsystemRoleBinding", func() {
		BeforeEach(func() {
			resources := []string{
				systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
				systemCapability("d8:system-capability:test2:edit", "others", "test2-ns"),
				systemRole("d8:subsystem:others:manager", "subsystem", "others"),
				manageBinding("test", "d8:subsystem:others:manager"),

				systemCapability("d8:system-capability:test3:edit", "test", "test2-ns"),
				systemRole("d8:subsystem:test:manager", "subsystem", "test"),
				manageBinding("test2", "d8:subsystem:test:manager"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should create RoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:namespace:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:test"))
			Expect(roleBinding.Field("roleRef.name").Str).To(Equal("d8:namespace:admin"))
			roleBinding = f.KubernetesResource("RoleBinding", "test2-ns", "d8:namespace:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:test"))

			roleBinding = f.KubernetesResource("RoleBinding", "test2-ns", "d8:namespace:admin:binding:test2")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:test2"))
		})
	})

	Context("There`s SystemRoleBinding", func() {
		BeforeEach(func() {
			resources := []string{
				systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
				systemCapability("d8:system-capability:test2:edit", "others", "test2-ns"),
				systemRole("d8:subsystem:others:manager", "subsystem", "others"),
				systemRole("d8:system:manager", "system", "system"),
				manageBinding("test", "d8:system:manager"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("Should create RoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:namespace:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:test"))
			roleBinding = f.KubernetesResource("RoleBinding", "test2-ns", "d8:namespace:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:test"))
		})
	})

	Context("There`s a SystemRoleBinding to a superadmin role (three aggregation levels)", func() {
		BeforeEach(func() {
			// d8:system:superadmin -> d8:subsystem:security:superadmin ->
			// d8:subsystem:security:manager -> capability (carries the namespace).
			// The namespaced capability sits three levels below the bound role.
			resources := []string{
				systemCapability("d8:system-capability:test:edit", "security", "sec-ns"),
				manageRoleWith("d8:subsystem:security:manager", "subsystem", "admin",
					map[string]string{"rbac.deckhouse.io/aggregate-to-security-as": "manager"},
					map[string]string{
						"rbac.deckhouse.io/subsystem":                "security",
						"rbac.deckhouse.io/aggregate-to-security-as": "superadmin",
						"rbac.deckhouse.io/aggregate-to-system-as":   "manager",
					}),
				manageRoleWith("d8:subsystem:security:superadmin", "subsystem", "superadmin",
					map[string]string{"rbac.deckhouse.io/aggregate-to-security-as": "superadmin"},
					map[string]string{
						"rbac.deckhouse.io/subsystem":              "security",
						"rbac.deckhouse.io/aggregate-to-system-as": "superadmin",
					}),
				manageRoleWith("d8:system:superadmin", "system", "superadmin",
					map[string]string{"rbac.deckhouse.io/aggregate-to-system-as": "superadmin"},
					nil),
				manageBinding("test", "d8:system:superadmin"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("fans out the namespaced binding through all aggregation levels", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "sec-ns", "d8:namespace:superadmin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:superadmin:binding:test"))
			Expect(roleBinding.Field("roleRef.name").Str).To(Equal("d8:namespace:superadmin"))
		})
	})

	Context("A namespace drops out of a manage binding", func() {
		BeforeEach(func() {
			resources := []string{
				// Binding "test" now resolves to a single namespace (test-ns).
				systemCapability("d8:system-capability:test:edit", "others", "test-ns"),
				systemRole("d8:subsystem:others:manager", "subsystem", "others"),
				manageBinding("test", "d8:subsystem:others:manager"),
				// Leftover RoleBinding from test2-ns, which no longer
				// contributes to the binding (its module capability lost the
				// rbac.deckhouse.io/namespace label or was removed).
				existingUseBinding("d8:namespace:admin:binding:test", "test2-ns"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("keeps the still-valid binding and deletes the orphan in the dropped namespace", func() {
			Expect(f).To(ExecuteSuccessfully())

			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:namespace:admin:binding:test")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:test"))

			orphan := f.KubernetesResource("RoleBinding", "test2-ns", "d8:namespace:admin:binding:test")
			Expect(orphan).To(BeEmpty())
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
			roleBinding := f.KubernetesResource("RoleBinding", "test-ns", "d8:binding:test")
			Expect(roleBinding).To(BeEmpty())
			roleBinding = f.KubernetesResource("RoleBinding", "test-ns", "d8:binding:test2")
			Expect(roleBinding).To(BeEmpty())
			roleBinding = f.KubernetesResource("RoleBinding", "test-ns2", "d8:binding:test3")
			Expect(roleBinding).To(BeEmpty())
			roleBinding = f.KubernetesResource("RoleBinding", "test-ns2", "d8:binding:test4")
			Expect(roleBinding).To(BeEmpty())
		})
	})

	Context("There`s a manage binding to a role aggregating via matchExpressions", func() {
		BeforeEach(func() {
			// The bound role selects its capability with a matchExpressions
			// selector (not matchLabels), which the previous matchLabels-only
			// traversal silently skipped.
			resources := []string{
				systemCapability("d8:system-capability:expr:edit", "exprtest", "expr-ns"),
				systemRoleWithExpressions("d8:system:exprmanager", "admin",
					"rbac.deckhouse.io/aggregate-to-exprtest-as", "manager"),
				manageBinding("exprbind", "d8:system:exprmanager"),
			}
			f.BindingContexts.Set(f.KubeStateSet(strings.Join(resources, "\n---\n")))
			f.RunHook()
		})

		It("traverses the expression-based aggregation and creates the RoleBinding", func() {
			Expect(f).To(ExecuteSuccessfully())
			roleBinding := f.KubernetesResource("RoleBinding", "expr-ns", "d8:namespace:admin:binding:exprbind")
			Expect(roleBinding.Field("metadata.name").Str).To(Equal("d8:namespace:admin:binding:exprbind"))
			Expect(roleBinding.Field("roleRef.name").Str).To(Equal("d8:namespace:admin"))
		})
	})
})

// systemRoleWithExpressions builds a manage ClusterRole whose AggregationRule
// uses a matchExpressions selector (key In values) instead of matchLabels.
func systemRoleWithExpressions(name, useRole, key string, values ...string) string {
	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"rbac.deckhouse.io/use-role": useRole,
				"rbac.deckhouse.io/kind":     "role",
				"rbac.deckhouse.io/scope":    "system",
			},
		},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: key, Operator: metav1.LabelSelectorOpIn, Values: values},
				},
			},
		}},
	}
	marshaled, _ := yaml.Marshal(&role)
	return string(marshaled)
}

func systemRole(name, scope, lineage string) string {
	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"rbac.deckhouse.io/use-role": "admin",
				"rbac.deckhouse.io/kind":     "role",
				"rbac.deckhouse.io/scope":    scope,
			},
		},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", lineage): "manager",
				},
			},
		}},
	}
	if scope == "subsystem" {
		role.Labels["rbac.deckhouse.io/subsystem"] = lineage
		role.Labels["rbac.deckhouse.io/aggregate-to-system-as"] = "manager"
	}
	marshaled, _ := yaml.Marshal(&role)
	return string(marshaled)
}

// manageRoleWith builds a manage ClusterRole (kind=role) with an explicit scope,
// use-role, the aggregation selector labels it matches (selects), and any extra
// labels it carries so higher-tier roles can aggregate it (carries).
func manageRoleWith(name, scope, useRole string, selects, carries map[string]string) string {
	labels := map[string]string{
		"rbac.deckhouse.io/use-role": useRole,
		"rbac.deckhouse.io/kind":     "role",
		"rbac.deckhouse.io/scope":    scope,
	}
	for k, v := range carries {
		labels[k] = v
	}
	selectors := make([]metav1.LabelSelector, 0, len(selects))
	for k, v := range selects {
		selectors = append(selectors, metav1.LabelSelector{MatchLabels: map[string]string{k: v}})
	}
	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		AggregationRule: &rbacv1.AggregationRule{ClusterRoleSelectors: selectors},
	}
	marshaled, _ := yaml.Marshal(&role)
	return string(marshaled)
}

func systemCapability(name, subsystem, namespace string) string {
	role := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"rbac.deckhouse.io/kind":                                       "capability",
				"rbac.deckhouse.io/scope":                                      "system",
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

func existingUseBinding(name, namespace string) string {
	binding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
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
			Name:     "d8:namespace:admin",
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
			Name:     "d8:namespace:user",
		},
	}
	marshaled, _ := yaml.Marshal(&binding)
	return string(marshaled)
}
