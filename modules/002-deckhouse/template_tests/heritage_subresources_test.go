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
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The heritage-subresources ValidationWebhook takes over where label-objects.deckhouse.io cannot
// look, so the two must protect against the same people: whoever may change a heritage object may
// exec into its pods and scale it, and nobody else -- with one exception of the webhook's own, the
// system:masters group, which is the way in on a master node while the fail-closed webhook is down.
// These tests evaluate the webhook matchConditions the way the API server does and compare the
// outcome with the policy for every identity below.

// webhookOnlyExemptGroup is the one exemption the webhook has beyond label-objects.
const webhookOnlyExemptGroup = "system:masters"

const heritageWebhookPath = "templates/webhook-handler/validating/heritage.yaml"

// identities lists who reaches the API server: cluster components, Deckhouse, the platform's own
// service accounts, cluster administrators by every route they come in, and plain users.
var identities = []struct {
	name     string
	username string
	groups   []string
}{
	{"a node", "system:node:worker-0", []string{"system:nodes"}},
	{"the HPA in kube-controller-manager", "system:serviceaccount:kube-system:horizontal-pod-autoscaler", []string{"system:serviceaccounts", "system:serviceaccounts:kube-system"}},
	{"kube-controller-manager", "system:kube-controller-manager", nil},
	{"kube-scheduler", "system:kube-scheduler", nil},
	{"volume-scheduler", "system:volume-scheduler", nil},
	{"kube-apiserver", "system:apiserver", []string{"system:masters"}},
	{"Deckhouse", "system:serviceaccount:d8-system:deckhouse", []string{"system:serviceaccounts", "system:serviceaccounts:d8-system"}},
	{"a d8-* service account", "system:serviceaccount:d8-monitoring:prometheus", []string{"system:serviceaccounts", "system:serviceaccounts:d8-monitoring"}},
	{"a kube-* service account", "system:serviceaccount:kube-public:reader", []string{"system:serviceaccounts", "system:serviceaccounts:kube-public"}},
	{"sudouser", "system:sudouser", []string{"system:sudousers"}},
	{"dhctl", "dhctl", nil},
	{"observability", "observability", nil},
	{"super-admin.conf", "kubernetes-super-admin", []string{"system:masters"}},
	{"any member of system:masters", "alice@example.com", []string{"system:masters"}},
	{"admin.conf", "kubernetes-admin", []string{"kubeadm:cluster-admins"}},
	{"a superadmin", "admin@example.com", []string{"superadmins"}},
	{"a user", "alice@example.com", nil},
	{"a user service account", "system:serviceaccount:team-a:ci", []string{"system:serviceaccounts", "system:serviceaccounts:team-a"}},
	{"a managed-service service account", "system:serviceaccount:d8ms-postgres:agent", []string{"system:serviceaccounts", "system:serviceaccounts:d8ms-postgres"}},
}

func TestHeritageWebhookExemptsWhatLabelObjectsExempts(t *testing.T) {
	policy := loadPolicy(t, "templates/validation.yaml", "exclude-restartedAt")
	conditions := loadWebhookConditions(t)

	for _, identity := range identities {
		t.Run(identity.name, func(t *testing.T) {
			request := updateRequest("Deployment", identity.username, identity.groups...)
			change := updated(func(o object) { container(o)["image"] = "example.com/injected:latest" })
			decision, detail := policy.evaluate(t, request, deployment(nil), change)
			if decision == denied && !strings.Contains(detail, "heritage") {
				t.Fatalf("label-objects denied for another reason: %s", detail)
			}
			mayChange := decision != denied
			// webhook = label-objects + system:masters, and nothing else.
			wantExempt := mayChange || slices.Contains(identity.groups, webhookOnlyExemptGroup)

			exempt := !webhookMatches(t, conditions, request)
			if exempt != wantExempt {
				t.Errorf("label-objects lets them change the object: %t, in %s: %t; the webhook exempts them: %t, want %t",
					mayChange, webhookOnlyExemptGroup, slices.Contains(identity.groups, webhookOnlyExemptGroup), exempt, wantExempt)
			}
		})
	}
}

// The extra exemption is the group alone: label-objects still denies a member of system:masters,
// and admin.conf, which kubeadm puts in kubeadm:cluster-admins, stays subject to the webhook.
func TestHeritageWebhookExtraExemptionIsSystemMastersOnly(t *testing.T) {
	policy := loadPolicy(t, "templates/validation.yaml", "exclude-restartedAt")
	conditions := loadWebhookConditions(t)
	change := func() object {
		return updated(func(o object) { container(o)["image"] = "example.com/injected:latest" })
	}

	masters := updateRequest("Deployment", "alice@example.com", webhookOnlyExemptGroup)
	if decision, _ := policy.evaluate(t, masters, deployment(nil), change()); decision != denied {
		t.Errorf("label-objects lets a member of %s change a heritage object: %s", webhookOnlyExemptGroup, decision)
	}
	if webhookMatches(t, conditions, masters) {
		t.Errorf("the webhook does not exempt a member of %s", webhookOnlyExemptGroup)
	}

	adminConf := updateRequest("Deployment", "kubernetes-admin", "kubeadm:cluster-admins")
	if !webhookMatches(t, conditions, adminConf) {
		t.Error("the webhook exempts admin.conf (kubeadm:cluster-admins)")
	}
}

// A webhook matchCondition that fails to compile invalidates the whole configuration the handler
// registers, every other hook on it included, so each has to compile to a bool on its own.
func TestHeritageWebhookConditionsCompile(t *testing.T) {
	if len(loadWebhookConditions(t)) != 4 {
		t.Fatal("the webhook has lost one of its four exemptions")
	}
}

func TestHeritageWebhookRules(t *testing.T) {
	webhook := loadWebhook(t)

	if webhook.ValidationObject.FailurePolicy != "Fail" {
		t.Errorf("failurePolicy is %q, want Fail", webhook.ValidationObject.FailurePolicy)
	}

	// The scale rules name exactly the kinds the webhook-handler may read: a kind routed here
	// without that permission would have every scale of it denied with a read error.
	routed := map[string][]string{}
	var connect bool
	for _, rule := range webhook.ValidationObject.Rules {
		switch {
		case slices.Equal(rule.Operations, []string{"CONNECT"}):
			connect = slices.Equal(rule.APIGroups, []string{""}) &&
				slices.Equal(rule.Resources, []string{"pods/exec", "pods/attach", "pods/portforward"})
		case slices.Equal(rule.Operations, []string{"UPDATE"}):
			if len(rule.APIGroups) != 1 {
				t.Fatalf("a scale rule names %v groups, want one", rule.APIGroups)
			}
			routed[rule.APIGroups[0]] = append(routed[rule.APIGroups[0]], rule.Resources...)
		}
	}
	if !connect {
		t.Error("no rule for CONNECT on pods/exec, pods/attach and pods/portforward")
	}

	want := map[string][]string{
		"apps": {"deployments/scale", "statefulsets/scale", "replicasets/scale"},
		"":     {"replicationcontrollers/scale"},
	}
	if !maps.EqualFunc(routed, want, slices.Equal[[]string]) {
		t.Errorf("scale rules route %v, want %v", routed, want)
	}
}

// What the webhook leaves to the policies: `kubectl debug` is reviewed as the Pod itself, so the
// heritage binding of label-objects selects it by the Pod labels; and a Scale keeps the name of its
// parent, which is all d8ms-prefix looks at.
func TestPoliciesCoverTheirSubresources(t *testing.T) {
	tests := []struct {
		name string
		// matchCondition names the policy by a condition only it has.
		matchCondition string
		resource       string
	}{
		{name: "label-objects covers kubectl debug", matchCondition: "exclude-restartedAt", resource: "pods/ephemeralcontainers"},
		{name: "d8ms-prefix covers scale", matchCondition: "exclude-managed-serviceaccounts", resource: "*/scale"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !slices.Contains(policyResources(t, tt.matchCondition), tt.resource) {
				t.Errorf("the policy does not match %s", tt.resource)
			}
		})
	}
}

var _ = Describe("Module :: deckhouse :: heritage-subresources webhook ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	render := func(version string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.deckhouseVersion", version)
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	It("carries the handler from webhooks/validating/heritage.py", func() {
		render("test")

		webhook := f.KubernetesGlobalResource("ValidationWebhook", "heritage-subresources")
		Expect(webhook.Exists()).To(BeTrue())

		handler, err := os.ReadFile("../webhooks/validating/heritage.py")
		Expect(err).ShouldNot(HaveOccurred())
		// The block scalar keeps one trailing newline whatever the file ends with.
		Expect(strings.TrimSpace(webhook.Field("handler.python").String())).To(Equal(strings.TrimSpace(string(handler))))
	})

	It("is left out of a dev build, like the heritage binding of label-objects", func() {
		render("dev")

		Expect(f.KubernetesGlobalResource("ValidationWebhook", "heritage-subresources").Exists()).To(BeFalse())
		Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "heritage-label-objects.deckhouse.io").Exists()).To(BeFalse())
	})
})

type heritageWebhook struct {
	Kind             string `yaml:"kind"`
	ValidationObject struct {
		MatchConditions []struct {
			Name       string `yaml:"name"`
			Expression string `yaml:"expression"`
		} `yaml:"matchConditions"`
		Rules []struct {
			APIGroups  []string `yaml:"apiGroups"`
			Operations []string `yaml:"operations"`
			Resources  []string `yaml:"resources"`
			Scope      string   `yaml:"scope"`
		} `yaml:"rules"`
		FailurePolicy string `yaml:"failurePolicy"`
	} `yaml:"validationObject"`
}

// helmComment matches a Helm comment, which may span several lines that withoutHelmActions would
// otherwise keep as YAML.
var helmComment = regexp.MustCompile(`(?s){{-?\s*/\*.*?\*/\s*-?}}`)

func loadWebhook(t *testing.T) heritageWebhook {
	t.Helper()

	content, err := os.ReadFile("../" + heritageWebhookPath)
	if err != nil {
		t.Fatalf("reading %s: %v", heritageWebhookPath, err)
	}

	template := helmComment.ReplaceAllString(string(content), "")
	decoder := yaml.NewDecoder(strings.NewReader(withoutHelmActions(template)))
	for {
		var webhook heritageWebhook
		if err := decoder.Decode(&webhook); err != nil {
			t.Fatalf("no ValidationWebhook in %s (%v)", heritageWebhookPath, err)
		}
		if webhook.Kind == "ValidationWebhook" {
			return webhook
		}
	}
}

func loadWebhookConditions(t *testing.T) []namedProgram {
	t.Helper()

	env := celEnvironment(t)
	var conditions []namedProgram
	for _, condition := range loadWebhook(t).ValidationObject.MatchConditions {
		conditions = append(conditions, namedProgram{
			name:    condition.Name,
			program: compile(t, env, condition.Name, condition.Expression, cel.BoolType),
		})
	}
	return conditions
}

// webhookMatches reports whether the API server would call the hook: every matchCondition true.
func webhookMatches(t *testing.T, conditions []namedProgram, request object) bool {
	t.Helper()

	for _, condition := range conditions {
		matches, err := evalBool(condition.program, map[string]any{"request": request})
		if err != nil {
			t.Fatalf("the %s match condition errored: %v", condition.name, err)
		}
		if !matches {
			return false
		}
	}
	return true
}

// policyResources returns the resources the ValidatingAdmissionPolicy with the given matchCondition
// in templates/validation.yaml matches.
func policyResources(t *testing.T, matchCondition string) []string {
	t.Helper()

	content, err := os.ReadFile("../templates/validation.yaml")
	if err != nil {
		t.Fatalf("reading templates/validation.yaml: %v", err)
	}

	type document struct {
		Kind string `yaml:"kind"`
		Spec struct {
			MatchConstraints struct {
				ResourceRules []struct {
					Resources []string `yaml:"resources"`
				} `yaml:"resourceRules"`
			} `yaml:"matchConstraints"`
			MatchConditions []struct {
				Name string `yaml:"name"`
			} `yaml:"matchConditions"`
		} `yaml:"spec"`
	}

	decoder := yaml.NewDecoder(strings.NewReader(withoutHelmActions(string(content))))
	for {
		var doc document
		if err := decoder.Decode(&doc); err != nil {
			t.Fatalf("no ValidatingAdmissionPolicy with the %s match condition (%v)", matchCondition, err)
		}
		if doc.Kind != "ValidatingAdmissionPolicy" {
			continue
		}
		for _, condition := range doc.Spec.MatchConditions {
			if condition.Name != matchCondition {
				continue
			}
			resources := []string{}
			for _, rule := range doc.Spec.MatchConstraints.ResourceRules {
				resources = append(resources, rule.Resources...)
			}
			return resources
		}
	}
}
