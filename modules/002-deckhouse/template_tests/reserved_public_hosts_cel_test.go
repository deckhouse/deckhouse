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

var (
	allowed = verdict{matched: true, allowed: true}
	denied  = verdict{matched: true, allowed: false}
	skipped = verdict{matched: false, allowed: true}
)

// evaluatePolicy runs the CEL of a rendered ValidatingAdmissionPolicy the way the apiserver does:
// match conditions first, then the variables in declaration order, then the validation. Compiling
// against a bare CEL environment is enough to catch the mistakes worth catching here, syntax and
// operations that do not exist, without standing up an apiserver.
func evaluatePolicy(policy object_store.KubeObject, params, object, request map[string]interface{}) verdict {
	env, err := cel.NewEnv(
		// The apiserver puts the string extension in the base environment of every admission
		// policy, at version 0 up to Kubernetes 1.29 and version 2 after it
		// (k8s.io/apiserver/pkg/cel/environment/base.go). lowerAscii and substring are in both.
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
			return skipped
		}
	}

	variables := map[string]interface{}{}
	activation["variables"] = variables
	for _, variable := range policy.Field("spec.variables").Array() {
		variables[variable.Get("name").String()] = eval(variable.Get("expression").String())
	}

	allow := eval(policy.Field("spec.validations.0.expression").String()) == true
	if !allow {
		// The apiserver falls back to the static message when this one fails, which would hide
		// the hostname that caused the denial.
		Expect(eval(policy.Field("spec.validations.0.messageExpression").String())).
			To(ContainSubstring("is reserved for Deckhouse platform services"))
	}
	return verdict{matched: true, allowed: allow}
}

func ingressWithHosts(hosts ...string) map[string]interface{} {
	rules := make([]interface{}, 0, len(hosts))
	for _, host := range hosts {
		rules = append(rules, map[string]interface{}{"host": host})
	}
	return map[string]interface{}{"spec": map[string]interface{}{"rules": rules}}
}

func httpRouteWithHosts(hosts ...string) map[string]interface{} {
	hostnames := make([]interface{}, 0, len(hosts))
	for _, host := range hosts {
		hostnames = append(hostnames, host)
	}
	return map[string]interface{}{"spec": map[string]interface{}{"hostnames": hostnames}}
}

func listenerSetWithHosts(hosts ...string) map[string]interface{} {
	listeners := make([]interface{}, 0, len(hosts))
	for i, host := range hosts {
		listener := map[string]interface{}{"name": string(rune('a' + i)), "port": 443, "protocol": "HTTPS"}
		if host != "" {
			listener["hostname"] = host
		}
		listeners = append(listeners, listener)
	}
	return map[string]interface{}{"spec": map[string]interface{}{"listeners": listeners}}
}

func requestFrom(username string) map[string]interface{} {
	return map[string]interface{}{
		"userInfo": map[string]interface{}{
			"username": username,
			"groups":   []interface{}{"system:authenticated"},
		},
	}
}

// paramsFrom hands the policies the whole data map of the ConfigMap that was actually rendered, so
// that a key the template stops writing, or writes in a shape the CEL cannot read, shows up here
// rather than being papered over by a map the test built itself.
func paramsFrom(cm object_store.KubeObject) map[string]interface{} {
	Expect(cm.Exists()).To(BeTrue(), "the parameters ConfigMap should be rendered")
	data, ok := cm["data"].(map[string]interface{})
	Expect(ok).To(BeTrue(), "the parameters ConfigMap should carry a data map")
	return map[string]interface{}{"data": data}
}

var _ = Describe("Module :: deckhouse :: reserved public hosts :: CEL ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	var (
		ingressPolicy     object_store.KubeObject
		httpRoutePolicy   object_store.KubeObject
		listenerSetPolicy object_store.KubeObject
		params            map[string]interface{}

		// The deckhouse ModuleConfig section under test and the recorded snapshot, both empty for a
		// cluster that never set one. Contexts assign them in a BeforeEach; the render happens in
		// JustBeforeEach, after them.
		domainTemplate      string
		reservedPublicHosts string
		snapshot            string
	)

	BeforeEach(func() {
		domainTemplate = "%s.example.com"
		// Asked for explicitly, so that these contexts test Template mode on a branch that ships
		// either default.
		reservedPublicHosts = `{mode: Template}`
		snapshot = ""
	})

	JustBeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.ValuesSet("global.modules.publicDomainTemplate", domainTemplate)
		if reservedPublicHosts != "" {
			f.ValuesSetFromYaml("deckhouse.reservedPublicHosts", reservedPublicHosts)
		}
		if snapshot != "" {
			f.ValuesSetFromYaml("deckhouse.internal.reservedPublicHosts", snapshot)
		}
		f.HelmRender(WithAPIVersions(
			validatingAdmissionPolicyAPI,
			validatingAdmissionPolicyBindingAPI,
			httpRouteAPI,
			listenerSetAPI,
		))
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		ingressPolicy = f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
		httpRoutePolicy = f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy)
		listenerSetPolicy = f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsListenerSetPolicy)

		params = paramsFrom(f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName))
	})

	tenant := requestFrom("tenant@example.com")

	// hostCase is one hostname and the verdict every kind must reach for it. Running the same table
	// against all three policies is what keeps a tenant from finding one kind that is treated more
	// leniently than the others.
	type hostCase struct {
		host string
		want verdict
		why  string
	}

	expectSameOnEveryKind := func(cases []hostCase) {
		Expect(cases).ToNot(BeEmpty(), "a table that iterates nothing would report green")
		// A kind with no policy would otherwise be read as a policy with no validation, which every
		// request passes.
		Expect(ingressPolicy.Exists()).To(BeTrue(), reservedHostsIngressPolicy)
		Expect(httpRoutePolicy.Exists()).To(BeTrue(), reservedHostsHTTPRoutePolicy)
		Expect(listenerSetPolicy.Exists()).To(BeTrue(), reservedHostsListenerSetPolicy)

		for _, c := range cases {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts(c.host), tenant)).
				To(Equal(c.want), "Ingress host %q: %s", c.host, c.why)
			Expect(evaluatePolicy(httpRoutePolicy, params, httpRouteWithHosts(c.host), tenant)).
				To(Equal(c.want), "HTTPRoute hostname %q: %s", c.host, c.why)
			Expect(evaluatePolicy(listenerSetPolicy, params, listenerSetWithHosts(c.host), tenant)).
				To(Equal(c.want), "ListenerSet listener hostname %q: %s", c.host, c.why)
		}
	}

	Context("Template mode reserves the namespace the template carves out", func() {
		// The three verdicts the reservation is about, checked on Ingress alone so that a failure
		// here is the verdict and not a missing policy for some other kind.
		It("denies a single-label hostname the platform does not publish today", func() {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("shop.example.com"), tenant)).
				To(Equal(denied), "the template could render this for a service named shop, which is "+
					"what a hand-maintained list of the names Deckhouse happens to publish can never cover")
		})

		It("denies the wildcard form of the namespace", func() {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("*.example.com"), tenant)).
				To(Equal(denied), "a tenant holding this shadows every platform hostname at once, "+
					"which is worse than the single-host takeover the reservation is about")
		})

		It("denies a hostname written with a root dot", func() {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("CONSOLE.EXAMPLE.COM."), tenant)).
				To(Equal(denied), "the API server rejects a trailing dot on its own, but the policy "+
					"must not be the reason it is allowed")
		})

		It("allows a two-label hostname, the shape an ecosystem application takes", func() {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("app.ns.example.com"), tenant)).
				To(Equal(allowed), "applications.publicDomainTemplate puts the instance and its "+
					"namespace in two labels, so it can never collide with the platform's one")
		})

		It("decides by whether the template could have rendered the hostname", func() {
			expectSameOnEveryKind([]hostCase{
				{"console.example.com", denied, "a hostname the platform publishes today"},
				{"shop.example.com", denied,
					"a single label under the platform's domain, which the template could render for a " +
						"service named shop -- this is what the literal list could never cover"},
				{"my-console.example.com", denied, "a label may contain a dash, so the template could render it"},
				{"a.example.com", denied, "a single-character label is a label"},
				{"app.ns.example.com", allowed,
					"two labels before the tail: the shape applications.publicDomainTemplate renders, " +
						"which the platform template can never produce"},
				{"shop.example.org", allowed, "another domain entirely"},
				{"example.com", allowed, "the tail on its own is not in the namespace"},
				{"shop.sub.example.com", allowed, "one label too deep"},
			})
		})

		It("reserves the wildcard form of the namespace, which no label could match", func() {
			expectSameOnEveryKind([]hostCase{
				{"*.example.com", denied,
					"in Template mode a tenant holding this shadows every platform hostname at once"},
				{"*.sub.example.com", allowed, "a wildcard over a domain the platform does not own"},
				{"*.example.org", allowed, "another domain entirely"},
			})
		})

		It("compares the hostname however it is spelled", func() {
			expectSameOnEveryKind([]hostCase{
				{"Console.Example.COM", denied, "lowercased before comparing"},
				{"console.example.com.", denied, "the root dot is stripped before comparing"},
				{"CONSOLE.EXAMPLE.COM.", denied, "both at once"},
				{"*.EXAMPLE.com", denied, "the wildcard form is lowercased too"},
			})
		})

		It("denies a request that hides a reserved hostname among its own", func() {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("shop.example.org", "console.example.com"), tenant)).
				To(Equal(denied))
			Expect(evaluatePolicy(httpRoutePolicy, params, httpRouteWithHosts("shop.example.org", "console.example.com"), tenant)).
				To(Equal(denied))
			Expect(evaluatePolicy(listenerSetPolicy, params, listenerSetWithHosts("shop.example.org", "console.example.com"), tenant)).
				To(Equal(denied))
		})

		It("allows a request that claims no hostname at all", func() {
			Expect(evaluatePolicy(ingressPolicy, params, map[string]interface{}{"spec": map[string]interface{}{}}, tenant)).
				To(Equal(allowed))
			Expect(evaluatePolicy(httpRoutePolicy, params, map[string]interface{}{"spec": map[string]interface{}{}}, tenant)).
				To(Equal(allowed), "an HTTPRoute without hostnames inherits the listener's")
			Expect(evaluatePolicy(listenerSetPolicy, params, listenerSetWithHosts(""), tenant)).
				To(Equal(allowed), "a listener without a hostname accepts any")
		})

		It("never reaches the validation for the writers of the platform's own objects", func() {
			for _, username := range []string{
				"system:serviceaccount:d8-system:deckhouse",
				"system:serviceaccount:d8-user-authn:dex",
				"system:serviceaccount:kube-system:some-controller",
				"dhctl",
			} {
				Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("console.example.com"), requestFrom(username))).
					To(Equal(skipped), "user %q", username)
			}
		})

		It("reserves nothing when the parameters are there but say nothing", func() {
			for name, broken := range map[string]map[string]interface{}{
				"no data at all":     {"data": map[string]interface{}{}},
				"an empty pattern":   {"data": map[string]interface{}{"hostPattern": "", "hosts": ""}},
				"an empty host list": {"data": map[string]interface{}{"hosts": ""}},
			} {
				Expect(evaluatePolicy(ingressPolicy, broken, ingressWithHosts("console.example.com"), tenant)).
					To(Equal(allowed), "with %s the policy must not deny everything", name)
			}
		})
	})

	Context("Template mode with the %s inside the first label", func() {
		BeforeEach(func() {
			domainTemplate = "kube-%s.company.my"
		})

		It("derives the namespace from the whole template, prefix included", func() {
			expectSameOnEveryKind([]hostCase{
				{"kube-console.company.my", denied, "what the template renders for console"},
				{"kube-shop.company.my", denied, "what it would render for a service named shop"},
				{"kube-a-b.company.my", denied, "a label with a dash in it"},
				{"shop.company.my", allowed, "no kube- prefix, so outside the namespace"},
				{"kube-shop.company.org", allowed, "another domain"},
				{"kube-shop.sub.company.my", allowed, "one label too deep"},
				{"*.company.my", allowed,
					"the platform owns kube-* inside company.my, not company.my itself, and " +
						"kube-*.company.my is not a hostname any API server accepts"},
			})
		})
	})

	Context("Template mode without a publicDomainTemplate", func() {
		BeforeEach(func() {
			domainTemplate = ""
			reservedPublicHosts = `{mode: Template, additionalHosts: ["admin.example.com"]}`
		})

		It("reserves nothing but what the operator named, rather than everything", func() {
			expectSameOnEveryKind([]hostCase{
				{"admin.example.com", denied, "the operator asked for this one"},
				{"console.example.com", allowed, "the platform publishes nothing, so it owns nothing"},
				{"shop.example.com", allowed, "an empty pattern must reserve nothing, not match everything"},
				{"*.example.com", allowed, "there is no namespace to hold a wildcard over"},
			})
		})
	})

	Context("An operator adjusts the reservation under Template mode", func() {
		BeforeEach(func() {
			reservedPublicHosts = `{mode: Template, excludedServices: ["grafana", "shop"], additionalHosts: ["admin.corp.example.org", "prometheus.example.com"]}`
		})

		It("gives back exactly the hostnames the excluded names render to", func() {
			expectSameOnEveryKind([]hostCase{
				{"grafana.example.com", allowed, "excludedServices names the service, the template renders the hostname"},
				{"shop.example.com", allowed,
					"under Template a name the platform does not publish is still reserved, so " +
						"excluding it is how a workload gets its hostname back"},
				{"console.example.com", denied, "the rest of the namespace is untouched"},
				{"grafana.example.org", allowed, "the exclusion is a hostname, not a label"},
				{"admin.corp.example.org", denied, "additionalHosts reaches outside the namespace"},
				{"prometheus.example.com", denied,
					"named on both sides is not a contradiction: additionalHosts always reserves"},
			})
		})

		It("still denies a request that hides a reserved hostname behind a freed one", func() {
			Expect(evaluatePolicy(ingressPolicy, params, ingressWithHosts("grafana.example.com", "console.example.com"), tenant)).
				To(Equal(denied))
		})
	})

	Context("The upgrade grandfathered what tenants already served", func() {
		BeforeEach(func() {
			snapshot = `{recorded: true, hosts: ["shop.example.com"]}`
		})

		It("allows the recorded hostnames and nothing beyond them", func() {
			expectSameOnEveryKind([]hostCase{
				{"shop.example.com", allowed, "recorded before the reservation started"},
				{"store.example.com", denied, "not recorded, so still the platform's namespace"},
				{"console.example.com", denied, "a platform hostname is reserved however it got claimed"},
			})
		})
	})

	Context("List mode keeps the reservation the module shipped before", func() {
		BeforeEach(func() {
			reservedPublicHosts = `{mode: List}`
		})

		It("reserves the names the platform publishes and nothing else", func() {
			expectSameOnEveryKind([]hostCase{
				{"console.example.com", denied, "on the list"},
				{"kubeconfig.example.com", denied, "on the list"},
				{"Console.Example.COM", denied, "lowercased before comparing"},
				{"shop.example.com", allowed, "not a hostname the platform publishes"},
				{"my-console.example.com", allowed, "the match is exact, not a substring"},
				{"*.example.com", allowed, "wildcards were out of scope of the list"},
				{"app.ns.example.com", allowed, "not a hostname the platform publishes"},
			})
		})

		It("ignores a grandfathering snapshot, there is nothing to grandfather", func() {
			// Recorded so that a later switch to Template mode has it, but a recorded hostname must
			// not become an exception to a reservation that never covered it.
			f.ValuesSetFromYaml("deckhouse.internal.reservedPublicHosts", `{recorded: true, hosts: ["console.example.com"]}`)
			f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			Expect(evaluatePolicy(policy, paramsFrom(cm), ingressWithHosts("console.example.com"), tenant)).
				To(Equal(denied))
		})

		Context("with an exclusion", func() {
			BeforeEach(func() {
				reservedPublicHosts = `{mode: List, excludedServices: ["grafana"], additionalHosts: ["admin.example.com"]}`
			})

			It("frees the excluded hostname and reserves the added one", func() {
				expectSameOnEveryKind([]hostCase{
					{"grafana.example.com", allowed, "dropped from the list"},
					{"admin.example.com", denied, "added to the list"},
					{"console.example.com", denied, "the rest of the list is untouched"},
					{"shop.example.com", allowed, "an unpublished name was never reserved under List"},
				})
			})
		})
	})
})
