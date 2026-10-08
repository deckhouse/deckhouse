/*
Copyright 2026 Flant JSC

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

package template_tests

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/webhooks/validationwebhook"
)

// The ValidationWebhooks of the module and the files of webhooks/validating their handler is made
// of, in the order the chart joins them.
var userAuthzValidationWebhooks = map[string][]string{
	"user-authz-identity-collision-group":  {"webhooks/validating/identity_collision.py"},
	"user-authz-identity-collision-user":   {"webhooks/validating/identity_collision.py"},
	"user-authz-identity-privilege":        {"webhooks/validating/identity_assign.py", "webhooks/validating/identity_privilege.py"},
	"user-authz-multitenancy-car":          {"webhooks/validating/multitenancy.py"},
	"user-authz-multitenancy-moduleconfig": {"webhooks/validating/multitenancy.py"},
	"user-authz-rbacv2-cluster-roles":      {"webhooks/validating/cluster_roles.py"},
	"user-authz-rbacv2-role-escalation":    {"webhooks/validating/role_escalation.py"},
	"user-authz-role-binding":              {"webhooks/validating/role_binding.py"},
	"user-authz-system-resources-edit":     {"webhooks/validating/system_resources.py"},
	"user-authz-system-resources-exec":     {"webhooks/validating/system_resources.py"},
}

var _ = Describe("Module :: user-authz :: helm template :: validation webhooks", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	resource := func(name string) object_store.KubeObject {
		object := f.KubernetesGlobalResource("ValidationWebhook", name)
		Expect(object.Exists()).To(BeTrue(), name)
		return object
	}

	// binding returns the context binding of a ValidationWebhook by name.
	binding := func(webhook, name string) map[string]interface{} {
		for _, item := range resource(webhook).Field("context").Array() {
			if item.Get("name").String() == name {
				return item.Get("kubernetes").Value().(map[string]interface{})
			}
		}
		Fail(fmt.Sprintf("%s has no context binding %s", webhook, name))
		return nil
	}

	jq := func(webhook, name string, object interface{}) map[string]interface{} {
		result, err := validationwebhook.RunJQ(binding(webhook, name)["jqFilter"].(string), object)
		Expect(err).ShouldNot(HaveOccurred())
		return result.(map[string]interface{})
	}

	matchConditions := func(webhook string) map[string]string {
		conditions := map[string]string{}
		for _, condition := range resource(webhook).Field("validationObject.matchConditions").Array() {
			name := condition.Get("name").String()
			Expect(conditions).NotTo(HaveKey(name), "duplicate matchCondition name")
			conditions[name] = condition.Get("expression").String()
		}
		return conditions
	}

	expressions := func(webhook string) []string {
		var all []string
		for _, expression := range matchConditions(webhook) {
			all = append(all, expression)
		}
		return all
	}

	pythonStrings := func(file, name string) []string {
		values, err := validationwebhook.PythonStrings(moduleDir, file, name)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(values).NotTo(BeEmpty())
		return values
	}

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSet("userAuthz.enableMultiTenancy", false)
		f.ValuesSet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.key", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.key", "test")
		f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", true)
		f.HelmRender()
	})

	It("Should check every ValidationWebhook the module declares", func() {
		declared, err := validationwebhook.ManifestNames(moduleDir)
		Expect(err).ShouldNot(HaveOccurred())
		checked := make([]string, 0, len(userAuthzValidationWebhooks))
		for name := range userAuthzValidationWebhooks {
			checked = append(checked, name)
		}
		Expect(checked).To(ConsistOf(declared))
	})

	It("Should render the handlers from webhooks/validating", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name, files := range userAuthzValidationWebhooks {
			handler := resource(name).Field("handler.python").String()
			Expect(validationwebhook.HandlerMatchesFiles(moduleDir, handler, files...)).To(Succeed(), name)
		}
	})

	It("Should render only the fields the CRD and the operator template accept", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name := range userAuthzValidationWebhooks {
			object := resource(name)
			undeclared, err := validationwebhook.UndeclaredFields(object)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(undeclared).To(BeEmpty(), name)
			Expect(validationwebhook.LiteralHazards(object)).To(BeEmpty(), name)
		}
	})

	It("Should keep the wrapper of the python test helper in sync with the operator template", func() {
		Expect(validationwebhook.TestHelperMatchesTemplate(moduleDir)).To(Succeed())
	})

	Context("identity-collision", func() {
		It("Should exclude the identities the hook exempts in matchConditions of both webhooks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			users := pythonStrings("webhooks/validating/identity_collision.py", "EXEMPT_USERS")
			groups := pythonStrings("webhooks/validating/identity_collision.py", "EXEMPT_GROUPS")
			for _, webhook := range []string{"user-authz-identity-collision-group", "user-authz-identity-collision-user"} {
				all := expressions(webhook)
				Expect(all).To(HaveLen(len(users) + len(groups)))
				for _, user := range users {
					Expect(all).To(ContainElement(fmt.Sprintf(`("%s" != request.userInfo.username)`, user)), webhook)
				}
				for _, group := range groups {
					Expect(all).To(ContainElement(fmt.Sprintf(`!("%s" in request.userInfo.groups)`, group)), webhook)
				}
			}
		})

		It("Should declare the same two rule snapshots on both webhooks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			group := resource("user-authz-identity-collision-group").Field("context").Value()
			user := resource("user-authz-identity-collision-user").Field("context").Value()
			Expect(group).To(Equal(user))

			var names []string
			for _, item := range resource("user-authz-identity-collision-group").Field("context").Array() {
				names = append(names, item.Get("name").String())
			}
			Expect(names).To(ConsistOf(
				"d8-user-authz-collision-cluster-authorization-rules",
				"d8-user-authz-collision-authorization-rules",
			))
		})

		mixedSubjects := []interface{}{
			map[string]interface{}{"kind": "Group", "name": "admins"},
			map[string]interface{}{"kind": "User", "name": "admin@example.com"},
			map[string]interface{}{"kind": "ServiceAccount", "name": "builder", "namespace": "ci"},
		}

		It("Should keep only the group and user subjects of a ClusterAuthorizationRule", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			out := jq("user-authz-identity-collision-group", "d8-user-authz-collision-cluster-authorization-rules", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "admin-rule"},
				"spec":     map[string]interface{}{"subjects": mixedSubjects},
			})
			Expect(out).To(Equal(map[string]interface{}{
				"name":          "admin-rule",
				"groupSubjects": []interface{}{"admins"},
				"userSubjects":  []interface{}{"admin@example.com"},
			}))
		})

		It("Should keep only the group and user subjects of an AuthorizationRule", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			out := jq("user-authz-identity-collision-group", "d8-user-authz-collision-authorization-rules", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "team-rule", "namespace": "team-a"},
				"spec":     map[string]interface{}{"subjects": mixedSubjects},
			})
			Expect(out).To(Equal(map[string]interface{}{
				"name":          "team-rule",
				"namespace":     "team-a",
				"groupSubjects": []interface{}{"admins"},
				"userSubjects":  []interface{}{"admin@example.com"},
			}))
		})

		It("Should tolerate a rule without subjects", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, name := range []string{"d8-user-authz-collision-cluster-authorization-rules", "d8-user-authz-collision-authorization-rules"} {
				out := jq("user-authz-identity-collision-group", name, map[string]interface{}{
					"metadata": map[string]interface{}{"name": "empty", "namespace": "team-a"},
					"spec":     map[string]interface{}{},
				})
				Expect(out["groupSubjects"]).To(Equal([]interface{}{}), name)
				Expect(out["userSubjects"]).To(Equal([]interface{}{}), name)
			}
		})
	})

	Context("identity-privilege", func() {
		const webhook = "user-authz-identity-privilege"

		rulesFor := func(resourceName string) []string {
			var operations []string
			for _, rule := range resource(webhook).Field("validationObject.rules").Array() {
				for _, r := range rule.Get("resources").Array() {
					if r.String() == resourceName {
						for _, operation := range rule.Get("operations").Array() {
							operations = append(operations, operation.String())
						}
					}
				}
			}
			sort.Strings(operations)
			return operations
		}

		It("Should validate the identity resources with the expected operations", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			var resources []string
			for _, rule := range resource(webhook).Field("validationObject.rules").Array() {
				for _, r := range rule.Get("resources").Array() {
					resources = append(resources, r.String())
				}
			}
			Expect(resources).To(ConsistOf(
				"clusterauthorizationrules", "clusterroles", "dexproviders", "groups", "useroperations", "users",
			))
			Expect(rulesFor("useroperations")).To(Equal([]string{"CREATE"}))
			Expect(rulesFor("clusterauthorizationrules")).To(Equal([]string{"CREATE", "DELETE", "UPDATE"}))
		})

		It("Should read the snapshots the kernel uses", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			var kinds []string
			for _, item := range resource(webhook).Field("context").Array() {
				kinds = append(kinds, item.Get("kubernetes.kind").String())
			}
			Expect(kinds).To(Equal([]string{
				"ClusterAuthorizationRule", "AuthorizationRule",
				"ClusterRoleBinding", "ClusterProjectRoleBinding", "ProjectRoleBinding", "ClusterRole",
				"User", "Group", "ConfigMap",
			}))

			// v1alpha1 has no namespaceSelector, and the API server drops the field from an object it
			// serves in v1alpha1.
			Expect(binding(webhook, "d8-user-authz-assign-cluster-authorization-rules")["apiVersion"]).To(Equal("deckhouse.io/v1"))

			state := resource(webhook)
			for _, item := range state.Field("context").Array() {
				if item.Get("name").String() == "d8-user-authz-assign-multitenancy-state" {
					Expect(item.Get("kubernetes.namespace.nameSelector.matchNames").Value()).To(Equal([]interface{}{"d8-user-authz"}))
					Expect(item.Get("kubernetes.nameSelector.matchNames").Value()).To(Equal([]interface{}{"d8-user-authz-multitenancy-state"}))
				}
			}
		})

		It("Should name the snapshots the way the kernel reads them", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			var names []string
			for _, item := range resource(webhook).Field("context").Array() {
				names = append(names, item.Get("name").String())
			}
			var kernel []string
			for _, constant := range []string{"CAR_SNAP", "AR_SNAP", "CRB_SNAP", "CPRB_SNAP", "PRB_SNAP", "CROLE_SNAP", "USER_SNAP", "GROUP_SNAP", "MT_STATE_SNAP"} {
				kernel = append(kernel, pythonConstant(moduleDir, "webhooks/validating/identity_assign.py", constant))
			}
			Expect(names).To(ConsistOf(kernel))
		})

		It("Should keep the access level, the roles, the subjects and the namespace limits of a ClusterAuthorizationRule", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			out := jq(webhook, "d8-user-authz-assign-cluster-authorization-rules", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "admin-rule"},
				"spec": map[string]interface{}{
					"accessLevel":     "SuperAdmin",
					"additionalRoles": []interface{}{map[string]interface{}{"name": "cluster-admin"}},
					"subjects": []interface{}{
						map[string]interface{}{"kind": "Group", "name": "g"},
						map[string]interface{}{"kind": "ServiceAccount", "name": "sa", "namespace": "ns"},
						map[string]interface{}{"kind": "User", "name": "u@x"},
					},
				},
			})
			Expect(out["accessLevel"]).To(Equal("SuperAdmin"))
			Expect(out["additionalRoles"]).To(Equal([]interface{}{"cluster-admin"}))
			Expect(out["saSubjects"]).To(Equal([]interface{}{"ns:sa"}))
			Expect(out["groupSubjects"]).To(Equal([]interface{}{"g"}))
			Expect(out["userSubjects"]).To(Equal([]interface{}{"u@x"}))
			Expect(out).To(HaveKeyWithValue("namespaceSelector", BeNil()))
			Expect(out["limitNamespaces"]).To(Equal([]interface{}{}))

			selector := map[string]interface{}{"labelSelector": map[string]interface{}{"matchLabels": map[string]interface{}{"team": "apps"}}}
			limited := jq(webhook, "d8-user-authz-assign-cluster-authorization-rules", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "limited"},
				"spec": map[string]interface{}{
					"accessLevel":       "User",
					"namespaceSelector": selector,
					"limitNamespaces":   []interface{}{"app-.*"},
				},
			})
			Expect(limited["namespaceSelector"]).To(Equal(selector))
			Expect(limited["limitNamespaces"]).To(Equal([]interface{}{"app-.*"}))

			for spec, expected := range map[string]bool{`{}`: false, `{"allowAccessToSystemNamespaces": false}`: false, `{"allowAccessToSystemNamespaces": true}`: true} {
				out := jq(webhook, "d8-user-authz-assign-cluster-authorization-rules", map[string]interface{}{
					"metadata": map[string]interface{}{"name": "r"},
					"spec":     parseJSON(spec),
				})
				Expect(out["allowAccessToSystemNamespaces"]).To(Equal(expected), spec)
			}
		})

		It("Should read the multitenancy state as true, false or unknown", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for data, expected := range map[string]interface{}{
				`{"enableMultiTenancy": "true"}`:  true,
				`{"enableMultiTenancy": "false"}`: false,
				`{"enableMultiTenancy": "yes"}`:   nil,
				`{}`:                              nil,
			} {
				out := jq(webhook, "d8-user-authz-assign-multitenancy-state", map[string]interface{}{
					"metadata": map[string]interface{}{"name": "s"},
					"data":     parseJSON(data),
				})
				Expect(out).To(Equal(map[string]interface{}{"enableMultiTenancy": expected}), data)
			}
		})

		It("Should keep the role and the subjects of the bindings", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			out := jq(webhook, "d8-user-authz-assign-cluster-role-bindings", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "b"},
				"roleRef":  map[string]interface{}{"name": "d8:manage:security:manager"},
				"subjects": []interface{}{map[string]interface{}{"kind": "User", "name": "sec@corp"}},
			})
			Expect(out["role"]).To(Equal("d8:manage:security:manager"))
			Expect(out["userSubjects"]).To(Equal([]interface{}{"sec@corp"}))

			for _, name := range []string{
				pythonConstant(moduleDir, "webhooks/validating/identity_assign.py", "CPRB_SNAP"),
				pythonConstant(moduleDir, "webhooks/validating/identity_assign.py", "PRB_SNAP"),
			} {
				out := jq(webhook, name, map[string]interface{}{
					"metadata": map[string]interface{}{"name": "team-admins", "namespace": "team"},
					"spec": map[string]interface{}{
						"roleRef": map[string]interface{}{"kind": "ClusterRole", "name": "d8:project:admin"},
						"subjects": []interface{}{
							map[string]interface{}{"kind": "User", "name": "padmin@corp"},
							map[string]interface{}{"kind": "Group", "name": "team-leads"},
							map[string]interface{}{"kind": "ServiceAccount", "name": "deployer", "namespace": "ci"},
						},
					},
				})
				Expect(out["role"]).To(Equal("d8:project:admin"), name)
				Expect(out["namespace"]).To(Equal("team"), name)
				Expect(out["userSubjects"]).To(Equal([]interface{}{"padmin@corp"}), name)
				Expect(out["groupSubjects"]).To(Equal([]interface{}{"team-leads"}), name)
				Expect(out["saSubjects"]).To(Equal([]interface{}{"ci:deployer"}), name)
			}
		})

		It("Should keep the can-assign labels and the rules of a ClusterRole", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			rules := []interface{}{map[string]interface{}{
				"apiGroups": []interface{}{"deckhouse.io"},
				"resources": []interface{}{"clusterauthorizationrules"},
				"verbs":     []interface{}{"create", "update", "patch", "delete"},
			}}
			out := jq(webhook, pythonConstant(moduleDir, "webhooks/validating/identity_assign.py", "CROLE_SNAP"), map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "d8:manage:security:manager",
					"labels": map[string]interface{}{
						"user-authz.deckhouse.io/can-assign-basic-max": "ClusterAdmin",
						"user-authz.deckhouse.io/can-assign-max-level": "admin",
					},
				},
				"rules": rules,
			})
			Expect(out["labels"]).To(HaveKeyWithValue("can-assign-basic-max", "ClusterAdmin"))
			Expect(out["rules"]).To(Equal(rules))
		})

		It("Should keep the email, the groups and the members of users and groups", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			user := jq(webhook, "d8-user-authz-assign-users", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "admin"},
				"spec":     map[string]interface{}{"email": "admin@deckhouse.io", "groups": []interface{}{"legacy"}},
			})
			Expect(user["email"]).To(Equal("admin@deckhouse.io"))
			Expect(user["groups"]).To(Equal([]interface{}{"legacy"}))

			members := []interface{}{
				map[string]interface{}{"kind": "User", "name": "admin"},
				map[string]interface{}{"kind": "Group", "name": "other"},
			}
			group := jq(webhook, "d8-user-authz-assign-groups", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "g1"},
				"spec":     map[string]interface{}{"name": "superadmins", "members": members},
			})
			Expect(group["name"]).To(Equal("superadmins"))
			Expect(group["members"]).To(Equal(members))
		})
	})

	Context("multitenancy", func() {
		It("Should give the ClusterAuthorizationRule webhook the multitenancy state only", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			car := resource("user-authz-multitenancy-car")
			Expect(car.Field("validationObject.name").String()).To(Equal("d8-user-authz-car-multitenancy-related-options.deckhouse.io"))
			// shell-operator unions includeSnapshotsFrom with every binding of the webhook's group, and
			// the operator puts every context binding into the group "main".
			Expect(car.Field("validationObject.group").Exists()).To(BeFalse())
			Expect(car.Field("context.#.name").Value()).To(Equal([]interface{}{"d8-user-authz-multitenancy-state"}))
		})

		It("Should give the ModuleConfig webhook the state and the rules, read in v1", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			moduleConfig := resource("user-authz-multitenancy-moduleconfig")
			Expect(moduleConfig.Field("validationObject.name").String()).To(Equal("d8-user-authz-module-multitenancy-related-options.deckhouse.io"))
			Expect(moduleConfig.Field("validationObject.group").Exists()).To(BeFalse())
			Expect(moduleConfig.Field("context.#.name").Value()).To(ConsistOf("d8-user-authz-multitenancy-state", "d8-user-authz-cars"))

			// v1alpha1 has no namespaceSelector, and the API server drops the field from an object it
			// serves in v1alpha1, so a rule that sets only namespaceSelector would pass unnoticed.
			rules := binding("user-authz-multitenancy-moduleconfig", "d8-user-authz-cars")
			Expect(rules["apiVersion"]).To(Equal("deckhouse.io/v1"))
			Expect(rules["kind"]).To(Equal("ClusterAuthorizationRule"))
		})

		It("Should keep the name and every restricted field of a rule, a null one included", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			out := jq("user-authz-multitenancy-moduleconfig", "d8-user-authz-cars", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "restricted", "uid": "c0ffee00"},
				"spec": map[string]interface{}{
					"accessLevel":                   "User",
					"allowScale":                    true,
					"allowAccessToSystemNamespaces": true,
					"limitNamespaces":               []interface{}{"team-.*"},
					"namespaceSelector":             nil,
				},
			})
			Expect(out).To(Equal(map[string]interface{}{
				"metadata": map[string]interface{}{"name": "restricted"},
				"spec": map[string]interface{}{
					"allowAccessToSystemNamespaces": true,
					"limitNamespaces":               []interface{}{"team-.*"},
					"namespaceSelector":             nil,
				},
			}))

			plain := jq("user-authz-multitenancy-moduleconfig", "d8-user-authz-cars", map[string]interface{}{
				"metadata": map[string]interface{}{"name": "plain"},
			})
			Expect(plain).To(Equal(map[string]interface{}{
				"metadata": map[string]interface{}{"name": "plain"},
				"spec":     map[string]interface{}{},
			}))
		})
	})

	Context("system-resources", func() {
		// Both webhooks are fail-closed and the exec one matches every pod in the cluster, so the
		// bypass has to be evaluated by the API server: an unavailable webhook-handler must not block
		// exec for the cluster administrators trying to diagnose it.
		It("Should exclude the bypass groups and the privileged users in matchConditions of both webhooks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			groups := pythonStrings("webhooks/validating/system_resources.py", "BYPASS_GROUPS")
			users := pythonStrings("webhooks/validating/system_resources.py", "PRIVILEGED_USERS")
			for _, webhook := range []string{"user-authz-system-resources-edit", "user-authz-system-resources-exec"} {
				all := expressions(webhook)
				for _, group := range groups {
					Expect(all).To(ContainElement(fmt.Sprintf(`!("%s" in request.userInfo.groups)`, group)), webhook)
				}
				for _, user := range users {
					Expect(all).To(ContainElement(fmt.Sprintf(`"%s" != request.userInfo.username`, user)), webhook)
				}
			}
		})

		// The edit webhook matches every namespaced UPDATE and DELETE, so its only-marked-objects
		// condition is what keeps ordinary objects off the handler. It has to look for the very
		// markings the hook checks, or a renamed label would leave the protection never called.
		It("Should pre-filter the edit webhook on the markings the hook checks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			expression := matchConditions("user-authz-system-resources-edit")["only-marked-objects"]
			Expect(expression).NotTo(BeEmpty())
			file := "webhooks/validating/system_resources.py"
			label := pythonConstant(moduleDir, file, "SYSTEM_RESOURCE_LABEL")
			value := pythonConstant(moduleDir, file, "SYSTEM_RESOURCE_VALUE")
			heritage := pythonConstant(moduleDir, file, "HERITAGE_LABEL")
			multitenancy := pythonConstant(moduleDir, file, "HERITAGE_MULTITENANCY")
			for _, object := range []string{"object", "oldObject"} {
				Expect(expression).To(ContainSubstring(fmt.Sprintf(
					"('%s' in %s.metadata.labels && %s.metadata.labels['%s'] == '%s')", label, object, object, label, value)))
				Expect(expression).To(ContainSubstring(fmt.Sprintf(
					"('%s' in %s.metadata.labels && %s.metadata.labels['%s'] == '%s')", heritage, object, object, heritage, multitenancy)))
			}
		})

		It("Should name the webhooks the way the hook dispatches on them", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(resource("user-authz-system-resources-edit").Field("validationObject.name").String()).
				To(Equal(pythonConstant(moduleDir, "webhooks/validating/system_resources.py", "BINDING_EDIT")))
			Expect(resource("user-authz-system-resources-exec").Field("validationObject.name").String()).
				To(Equal(pythonConstant(moduleDir, "webhooks/validating/system_resources.py", "BINDING_EXEC")))
		})
	})
})

func pythonConstant(moduleDir, file, name string) string {
	value, err := validationwebhook.PythonString(moduleDir, file, name)
	Expect(err).ShouldNot(HaveOccurred())
	return value
}

func parseJSON(text string) interface{} {
	var value interface{}
	Expect(json.Unmarshal([]byte(text), &value)).To(Succeed())
	return value
}
