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

package decision

import (
	"os"
	"strings"
	"testing"
)

// authorizationConfigPath is the source of truth for who the webhook is asked about.
const authorizationConfigPath = "../../../modules/040-control-plane-manager/templates/_authorization_config.tpl"

// The list in exempt.go is a copy of the CEL in the AuthorizationConfiguration, and a copy that
// nothing checks is a copy that drifts. This reads the template and fails when the two stop
// agreeing, in either direction: a condition added there and not here makes permission-browser
// report limits the API server does not enforce, and one removed there and not here makes it report
// freedom the API server does not grant.
//
// It compares the expressions verbatim rather than evaluating them. The point is to stop a change
// in the template from landing silently, and a human updating both sides is the whole mechanism.
func TestExemptionsMirrorTheAuthorizationConfig(t *testing.T) {
	raw, err := os.ReadFile(authorizationConfigPath)
	if err != nil {
		// This module is copied on its own into the image builds, where the rest of the repository
		// is not there to read. Nothing to compare against then - but only then: if the tree IS
		// here and the file is not, the path has moved and the check has to be noticed, not
		// skipped.
		if _, repo := os.Stat("../../../modules"); repo != nil {
			t.Skipf("the repository tree is not here, nothing to compare against: %v", err)
		}
		t.Fatalf("reading the authorization config template: %v", err)
	}

	want := []string{
		`'!(request.user in ["system:aggregator", "system:kube-aggregator", "system:kube-controller-manager", "system:kube-scheduler", "kubernetes-admin", "kube-apiserver-kubelet-client", "capi-controller-manager", "system:volume-scheduler"])'`,
		`'!(request.user.startsWith("system:node:"))'`,
		`'!(request.user.startsWith("system:serviceaccount:kube-system:"))'`,
		`'!(request.user.startsWith("system:serviceaccount:d8-"))'`,
	}

	var got []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if expression, ok := strings.CutPrefix(line, "- expression: "); ok {
			got = append(got, strings.TrimSpace(expression))
		}
	}

	if len(got) != len(want) {
		t.Fatalf("the template has %d matchConditions, this package mirrors %d.\nIf you changed the template, update exemptUsers/exemptPrefixes in exempt.go to match.\ntemplate:\n\t%s",
			len(got), len(want), strings.Join(got, "\n\t"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("matchCondition %d changed.\n got: %s\nwant: %s\nUpdate exemptUsers/exemptPrefixes in exempt.go, then this expectation.", i, got[i], want[i])
		}
	}
}

func TestExemptFromWebhook(t *testing.T) {
	for _, tc := range []struct {
		username string
		exempt   bool
	}{
		// Named outright by the first condition.
		{"system:kube-controller-manager", true},
		{"system:kube-scheduler", true},
		{"kubernetes-admin", true},
		{"capi-controller-manager", true},
		{"system:volume-scheduler", true},
		{"system:aggregator", true},
		{"system:kube-aggregator", true},
		{"kube-apiserver-kubelet-client", true},

		// The prefixes.
		{"system:node:worker-1", true},
		{"system:serviceaccount:kube-system:coredns", true},
		{"system:serviceaccount:d8-system:deckhouse", true},
		{"system:serviceaccount:d8-user-authz:webhook", true},

		// Everybody else, including the near misses.
		{"user@example.com", false},
		{"system:serviceaccount:default:app", false},
		{"system:serviceaccount:my-d8-namespace:app", false},
		{"system:nodes", false},
		{"system:node", false},
		{"", false},
		// The condition matches the whole name, not a prefix of it.
		{"kubernetes-admin-2", false},
		{"system:kube-scheduler-shadow", false},
	} {
		if got := ExemptFromWebhook(tc.username); got != tc.exempt {
			t.Errorf("ExemptFromWebhook(%q) = %v, want %v", tc.username, got, tc.exempt)
		}
	}
}
