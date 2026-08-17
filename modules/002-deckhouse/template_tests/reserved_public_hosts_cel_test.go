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
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// verdict is what the apiserver would do with a request, reproduced from the rendered policy: a
// request the match conditions filter out never reaches the validation, and one that reaches it is
// admitted only when the validation expression holds.
type verdict struct {
	matched bool
	allowed bool
}

// evaluatePolicy runs the CEL of a rendered ValidatingAdmissionPolicy the way the apiserver does:
// match conditions first, then the variables in declaration order, then the validation. Compiling
// against a bare CEL environment is enough to catch the mistakes worth catching here, syntax and
// operations that do not exist, without standing up an apiserver.
func evaluatePolicy(policy object_store.KubeObject, params, object, request map[string]interface{}) verdict {
	env, err := cel.NewEnv(
		ext.Strings(),
		cel.Variable("object", cel.DynType),
		cel.Variable("oldObject", cel.DynType),
		cel.Variable("request", cel.DynType),
		cel.Variable("params", cel.DynType),
		cel.Variable("variables", cel.DynType),
	)
	Expect(err).ShouldNot(HaveOccurred())

	activation := map[string]interface{}{
		"object":    object,
		"oldObject": nil,
		"request":   request,
		"params":    params,
		"variables": map[string]interface{}{},
	}

	eval := func(expression string) interface{} {
		ast, issues := env.Compile(expression)
		Expect(issues.Err()).ShouldNot(HaveOccurred(), "expression should compile: %s", expression)
		program, err := env.Program(ast)
		Expect(err).ShouldNot(HaveOccurred())
		out, _, err := program.Eval(activation)
		Expect(err).ShouldNot(HaveOccurred(), "expression should evaluate: %s", expression)
		return out.Value()
	}

	for _, condition := range policy.Field("spec.matchConditions").Array() {
		if eval(condition.Get("expression").String()) != true {
			return verdict{matched: false, allowed: true}
		}
	}

	variables := map[string]interface{}{}
	activation["variables"] = variables
	for _, variable := range policy.Field("spec.variables").Array() {
		variables[variable.Get("name").String()] = eval(variable.Get("expression").String())
	}

	allowed := eval(policy.Field("spec.validations.0.expression").String()) == true
	if !allowed {
		// The apiserver falls back to the static message when this one fails, which would hide
		// the hostname that caused the denial.
		Expect(eval(policy.Field("spec.validations.0.messageExpression").String())).
			To(ContainSubstring("is reserved for Deckhouse platform services"))
	}
	return verdict{matched: true, allowed: allowed}
}

func ingressWithHosts(hosts ...string) map[string]interface{} {
	rules := make([]interface{}, 0, len(hosts))
	for _, host := range hosts {
		rules = append(rules, map[string]interface{}{"host": host})
	}
	return map[string]interface{}{"spec": map[string]interface{}{"rules": rules}}
}

func requestFrom(username string) map[string]interface{} {
	return map[string]interface{}{
		"userInfo": map[string]interface{}{
			"username": username,
			"groups":   []interface{}{"system:authenticated"},
		},
	}
}

var _ = Describe("Module :: deckhouse :: reserved public hosts :: CEL ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	var (
		ingressPolicy   object_store.KubeObject
		httpRoutePolicy object_store.KubeObject
		params          map[string]interface{}
	)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		f.HelmRender(WithAPIVersions(
			validatingAdmissionPolicyAPI,
			validatingAdmissionPolicyBindingAPI,
			httpRouteAPI,
		))
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		ingressPolicy = f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
		httpRoutePolicy = f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy)

		cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
		params = map[string]interface{}{
			"data": map[string]interface{}{"hosts": cm.Field("data.hosts").String()},
		}
	})

	tenant := requestFrom("tenant@example.com")

	It("denies a tenant Ingress that claims a reserved hostname", func() {
		Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("console.example.com"), tenant)).
			To(Equal(verdict{matched: true, allowed: false}))
	})

	It("denies it whatever case the hostname is written in", func() {
		Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("Console.Example.COM"), tenant)).
			To(Equal(verdict{matched: true, allowed: false}))
	})

	It("denies an Ingress that hides the reserved hostname among its own", func() {
		object := ingressWithHosts("shop.example.com", "kubeconfig.example.com")
		Expect(evaluatePolicy(ingressPolicy, params, object, tenant)).
			To(Equal(verdict{matched: true, allowed: false}))
	})

	It("allows a tenant hostname the platform does not publish", func() {
		Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("shop.example.com"), tenant)).
			To(Equal(verdict{matched: true, allowed: true}))
	})

	It("allows a hostname that merely contains a reserved one", func() {
		Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("my-console.example.com"), tenant)).
			To(Equal(verdict{matched: true, allowed: true}))
	})

	It("allows an Ingress without any host, it claims nothing", func() {
		object := map[string]interface{}{"spec": map[string]interface{}{}}
		Expect(evaluatePolicy(ingressPolicy, params, object, tenant)).
			To(Equal(verdict{matched: true, allowed: true}))
	})

	It("never reaches the validation for a Deckhouse service account", func() {
		object := ingressWithHosts("console.example.com")
		Expect(evaluatePolicy(ingressPolicy, params, object, requestFrom("system:serviceaccount:d8-system:deckhouse"))).
			To(Equal(verdict{matched: false, allowed: true}))
	})

	It("never reaches the validation for a module's own service account", func() {
		object := ingressWithHosts("console.example.com")
		Expect(evaluatePolicy(ingressPolicy, params, object, requestFrom("system:serviceaccount:d8-user-authn:dex"))).
			To(Equal(verdict{matched: false, allowed: true}))
	})

	It("never reaches the validation for the installer", func() {
		object := ingressWithHosts("console.example.com")
		Expect(evaluatePolicy(ingressPolicy, params, object, requestFrom("dhctl"))).
			To(Equal(verdict{matched: false, allowed: true}))
	})

	It("allows everything when the reserved list is empty", func() {
		empty := map[string]interface{}{"data": map[string]interface{}{"hosts": ""}}
		Expect(evaluatePolicy(ingressPolicy, empty, ingressWithHosts("console.example.com"), tenant)).
			To(Equal(verdict{matched: true, allowed: true}))
	})

	It("allows everything when the parameters lost their key", func() {
		broken := map[string]interface{}{"data": map[string]interface{}{}}
		Expect(evaluatePolicy(ingressPolicy, broken, ingressWithHosts("console.example.com"), tenant)).
			To(Equal(verdict{matched: true, allowed: true}))
	})

	It("denies an HTTPRoute claiming a reserved hostname", func() {
		object := map[string]interface{}{"spec": map[string]interface{}{
			"hostnames": []interface{}{"dex.example.com"},
		}}
		Expect(evaluatePolicy(httpRoutePolicy, params, object, tenant)).
			To(Equal(verdict{matched: true, allowed: false}))
	})

	It("allows an HTTPRoute with no hostname, it inherits the listener's", func() {
		object := map[string]interface{}{"spec": map[string]interface{}{}}
		Expect(evaluatePolicy(httpRoutePolicy, params, object, tenant)).
			To(Equal(verdict{matched: true, allowed: true}))
	})
})
