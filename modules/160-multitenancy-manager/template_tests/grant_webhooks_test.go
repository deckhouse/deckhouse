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
	"fmt"

	"github.com/google/cel-go/cel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// grantWebhookJSON is the webhook the hooks used to create through the API, field for field, so the
// first render after the upgrade adopts the live configurations without changing what they do.
func grantWebhookJSON(name, path, rules string) string {
	return fmt.Sprintf(`{
  "name": %q,
  "clientConfig": {
    "caBundle": "dGVzdC1jYQ==",
    "service": {"name": "multitenancy-manager", "namespace": "d8-multitenancy-manager", "port": 9443, "path": %q}
  },
  "namespaceSelector": {"matchLabels": {"heritage": "multitenancy-manager"}},
  "matchConditions": [
    {"name": "exclude-apiserver", "expression": "request.userInfo.username != \"system:apiserver\""},
    {"name": "exclude-deckhouse-controller", "expression": "request.userInfo.username != \"system:serviceaccount:d8-system:deckhouse\""},
    {"name": "exclude-multitenancy-manager", "expression": "request.userInfo.username != \"system:serviceaccount:d8-multitenancy-manager:multitenancy-manager\""},
    {"name": "exclude-user-authz-controller", "expression": "request.userInfo.username != \"system:serviceaccount:d8-user-authz:controller\""},
    {"name": "exclude-system-serviceaccounts", "expression": "!request.userInfo.groups.exists(g, g == \"system:serviceaccounts:d8-system\" || g == \"system:serviceaccounts:kube-system\" || g == \"system:serviceaccounts:d8-user-authz\")"},
    {"name": "exclude-cluster-admins-and-nodes", "expression": "!request.userInfo.groups.exists(g, g == \"system:masters\" || g == \"system:nodes\")"}
  ],
  "sideEffects": "None",
  "admissionReviewVersions": ["v1"],
  "failurePolicy": "Fail",
  "timeoutSeconds": 10,
  "rules": %s
}`, name, path, rules)
}

// volumeClaimTemplatesCondition is the match condition that skips the objects without volume claim
// templates of the resources only the volume claim template references name.
const volumeClaimTemplatesCondition = "skip-objects-without-volume-claim-templates"

// matchesObject evaluates a rendered match condition the way the API server does for a request on the
// resource; an evaluation error fails the test, because under failurePolicy: Fail the API server would
// refuse the request.
func matchesObject(expression, group, resource string, object map[string]interface{}) bool {
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType), cel.Variable("request", cel.DynType))
	Expect(err).ShouldNot(HaveOccurred())
	ast, issues := env.Compile(expression)
	Expect(issues.Err()).ShouldNot(HaveOccurred(), "expression should compile: %s", expression)
	program, err := env.Program(ast)
	Expect(err).ShouldNot(HaveOccurred())
	out, _, err := program.Eval(map[string]interface{}{
		"object":  object,
		"request": map[string]interface{}{"resource": map[string]interface{}{"group": group, "version": "v1", "resource": resource}},
	})
	Expect(err).ShouldNot(HaveOccurred(), "expression should evaluate for %s/%s: %s", group, resource, expression)
	return out.Value() == true
}

func withVolumes(volumes ...interface{}) map[string]interface{} {
	return map[string]interface{}{"containers": []interface{}{map[string]interface{}{"name": "c"}}, "volumes": volumes}
}

var (
	emptyDirVolume  = map[string]interface{}{"name": "scratch", "emptyDir": map[string]interface{}{}}
	ephemeralVolume = map[string]interface{}{"name": "data", "ephemeral": map[string]interface{}{"volumeClaimTemplate": map[string]interface{}{"spec": map[string]interface{}{}}}}
)

func inPodTemplate(pod map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"spec": map[string]interface{}{"template": map[string]interface{}{"spec": pod}}}
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: grant webhooks ::", func() {
	const (
		defaultingName = "cluster-objects-grants-defaulting"
		validatorName  = "cluster-objects-grants-validator"
	)

	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.admissionWebhookCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.quotaLedgerCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ApplyOpenAPIDefaults()
	})

	Context("with grantable resources", func() {
		const rules = `[
  {"apiGroups": [""], "apiVersions": ["*"], "resources": ["persistentvolumeclaims"], "operations": ["CREATE", "UPDATE"], "scope": "Namespaced"},
  {"apiGroups": ["rbac.authorization.k8s.io"], "apiVersions": ["*"], "resources": ["rolebindings"], "operations": ["CREATE", "UPDATE"], "scope": "Namespaced"}
]`

		BeforeEach(func() {
			f.ValuesSetFromYaml("multitenancyManager.internal.grantWebhookRules", `[
  {"apiGroup": "", "resource": "persistentvolumeclaims"},
  {"apiGroup": "rbac.authorization.k8s.io", "resource": "rolebindings"}
]`)
			f.HelmRender()
		})

		It("renders the defaulting webhook as the hook created it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			wh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", defaultingName)
			Expect(wh.Exists()).To(BeTrue())
			Expect(wh.Field("webhooks").Array()).To(HaveLen(1))
			Expect(wh.Field("webhooks.0").String()).To(MatchJSON(grantWebhookJSON(
				defaultingName+".multitenancy.deckhouse.io", "/defaults", rules,
			)))
		})

		It("renders the validating webhook as the hook created it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			wh := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", validatorName)
			Expect(wh.Exists()).To(BeTrue())
			Expect(wh.Field("webhooks").Array()).To(HaveLen(1))
			Expect(wh.Field("webhooks.0").String()).To(MatchJSON(grantWebhookJSON(
				validatorName+".multitenancy.deckhouse.io", "/is-granted", rules,
			)))
		})

		It("makes both part of the module release, applied once the backend is ready", func() {
			for _, wh := range []struct{ kind, name string }{
				{kind: "MutatingWebhookConfiguration", name: defaultingName},
				{kind: "ValidatingWebhookConfiguration", name: validatorName},
			} {
				obj := f.KubernetesGlobalResource(wh.kind, wh.name)
				Expect(obj.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"), wh.name)
				Expect(obj.Field("metadata.labels.module").String()).To(Equal("multitenancy-manager"), wh.name)
				Expect(obj.Field(`metadata.annotations.werf\.io/deploy-dependency-deployment`).String()).To(
					Equal("state=ready,kind=Deployment,name=multitenancy-manager,namespace=d8-multitenancy-manager"), wh.name)
				Expect(obj.Field(`metadata.annotations.werf\.io/deploy-dependency-service`).String()).To(
					Equal("state=present,kind=Service,name=multitenancy-manager,namespace=d8-multitenancy-manager"), wh.name)
			}
		})
	})

	Context("with resources only the volume claim template references name", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("multitenancyManager.internal.grantWebhookRules", `[
  {"apiGroup": "", "resource": "persistentvolumeclaims"},
  {"apiGroup": "", "resource": "pods", "volumeClaimTemplatesOnly": true},
  {"apiGroup": "apps", "resource": "deployments", "volumeClaimTemplatesOnly": true},
  {"apiGroup": "apps", "resource": "statefulsets", "volumeClaimTemplatesOnly": true},
  {"apiGroup": "batch", "resource": "cronjobs", "volumeClaimTemplatesOnly": true}
]`)
			f.HelmRender()
		})

		condition := func(kind, name string) string {
			conditions := f.KubernetesGlobalResource(kind, name).Field("webhooks.0.matchConditions").Array()
			Expect(conditions).To(HaveLen(7), name)
			Expect(conditions[6].Get("name").String()).To(Equal(volumeClaimTemplatesCondition), name)
			return conditions[6].Get("expression").String()
		}

		It("still intercepts every resource and adds the condition to both webhooks", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			for _, wh := range []struct{ kind, name string }{
				{kind: "MutatingWebhookConfiguration", name: defaultingName},
				{kind: "ValidatingWebhookConfiguration", name: validatorName},
			} {
				Expect(f.KubernetesGlobalResource(wh.kind, wh.name).Field("webhooks.0.rules.#.resources.0").String()).To(
					MatchJSON(`["persistentvolumeclaims", "pods", "deployments", "statefulsets", "cronjobs"]`), wh.name)
				Expect(condition(wh.kind, wh.name)).To(Equal(condition("MutatingWebhookConfiguration", defaultingName)), wh.name)
			}
		})

		It("sends an object of such a resource only when it has a volume claim template", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			expression := condition("ValidatingWebhookConfiguration", validatorName)
			cases := []struct {
				title, group, resource string
				object                 map[string]interface{}
				matches                bool
			}{
				{"a pod without volumes", "", "pods", map[string]interface{}{"spec": map[string]interface{}{"containers": []interface{}{}}}, false},
				{"a pod with an emptyDir", "", "pods", map[string]interface{}{"spec": withVolumes(emptyDirVolume)}, false},
				{"a pod with an ephemeral volume", "", "pods", map[string]interface{}{"spec": withVolumes(emptyDirVolume, ephemeralVolume)}, true},
				{"a Deployment without volumes", "apps", "deployments", inPodTemplate(map[string]interface{}{"containers": []interface{}{}}), false},
				{"a Deployment with an emptyDir", "apps", "deployments", inPodTemplate(withVolumes(emptyDirVolume)), false},
				{"a Deployment with an ephemeral volume", "apps", "deployments", inPodTemplate(withVolumes(ephemeralVolume)), true},
				{"a StatefulSet without volume claim templates", "apps", "statefulsets", inPodTemplate(withVolumes(emptyDirVolume)), false},
				{"a StatefulSet with a volume claim template", "apps", "statefulsets", func() map[string]interface{} {
					sts := inPodTemplate(withVolumes(emptyDirVolume))
					sts["spec"].(map[string]interface{})["volumeClaimTemplates"] = []interface{}{map[string]interface{}{"metadata": map[string]interface{}{"name": "data"}}}
					return sts
				}(), true},
				{"a StatefulSet with an ephemeral volume", "apps", "statefulsets", inPodTemplate(withVolumes(ephemeralVolume)), true},
				{"a CronJob without volumes", "batch", "cronjobs", map[string]interface{}{"spec": map[string]interface{}{"jobTemplate": inPodTemplate(map[string]interface{}{"containers": []interface{}{}})}}, false},
				{"a CronJob with an ephemeral volume", "batch", "cronjobs", map[string]interface{}{"spec": map[string]interface{}{"jobTemplate": inPodTemplate(withVolumes(ephemeralVolume))}}, true},
				{"a PersistentVolumeClaim", "", "persistentvolumeclaims", map[string]interface{}{"spec": map[string]interface{}{}}, true},
				{"a Job, which the condition does not list", "batch", "jobs", inPodTemplate(map[string]interface{}{"containers": []interface{}{}}), true},
			}
			for _, c := range cases {
				Expect(matchesObject(expression, c.group, c.resource, c.object)).To(Equal(c.matches), c.title)
			}
		})
	})

	Context("without grantable resources", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("renders both webhooks with an empty rule list", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			for _, wh := range []struct{ kind, name string }{
				{kind: "MutatingWebhookConfiguration", name: defaultingName},
				{kind: "ValidatingWebhookConfiguration", name: validatorName},
			} {
				rules := f.KubernetesGlobalResource(wh.kind, wh.name).Field("webhooks.0.rules")
				Expect(rules.IsArray()).To(BeTrue(), wh.name)
				Expect(rules.Array()).To(BeEmpty(), wh.name)
			}
		})
	})

	Context("before the webhook certificate is issued", func() {
		BeforeEach(func() {
			f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "")
			f.HelmRender()
		})

		It("renders neither webhook", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("MutatingWebhookConfiguration", defaultingName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", validatorName).Exists()).To(BeFalse())
		})
	})
})
