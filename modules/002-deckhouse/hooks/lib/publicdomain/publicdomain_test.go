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

package publicdomain

import (
	"os"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestParseNamespacePattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		domainTemplate string
		pattern        string
		wildcard       string
	}{
		{
			name:           "the %s is the whole first label",
			domainTemplate: "%s.example.com",
			pattern:        `^[a-z0-9]([-a-z0-9]*[a-z0-9])?\.example\.com$`,
			wildcard:       "*.example.com",
		},
		{
			name:           "the %s shares its label with a prefix",
			domainTemplate: "kube-%s.company.my",
			pattern:        `^kube-[a-z0-9]([-a-z0-9]*[a-z0-9])?\.company\.my$`,
		},
		{
			name:           "the %s shares its label with a suffix",
			domainTemplate: "%s-kube.company.my",
			pattern:        `^[a-z0-9]([-a-z0-9]*[a-z0-9])?-kube\.company\.my$`,
		},
		{
			name:           "the %s is written in the middle of its label",
			domainTemplate: "pre-%s-post.example.com",
			pattern:        `^pre-[a-z0-9]([-a-z0-9]*[a-z0-9])?-post\.example\.com$`,
		},
		{
			name:           "the tail has several labels",
			domainTemplate: "%s.kube.a-b.company.my",
			pattern:        `^[a-z0-9]([-a-z0-9]*[a-z0-9])?\.kube\.a-b\.company\.my$`,
			wildcard:       "*.kube.a-b.company.my",
		},
		{
			name:           "the template is written in upper case",
			domainTemplate: "%s.EXAMPLE.COM",
			pattern:        `^[a-z0-9]([-a-z0-9]*[a-z0-9])?\.example\.com$`,
			wildcard:       "*.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			namespace, err := ParseNamespace(tc.domainTemplate)
			if err != nil {
				t.Fatalf("ParseNamespace(%q): %v", tc.domainTemplate, err)
			}
			if got := namespace.Pattern.String(); got != tc.pattern {
				t.Errorf("pattern = %q, want %q", got, tc.pattern)
			}
			if namespace.Wildcard != tc.wildcard {
				t.Errorf("wildcard = %q, want %q", namespace.Wildcard, tc.wildcard)
			}
		})
	}
}

func TestParseNamespaceRejectsWhatTheSchemaRejects(t *testing.T) {
	t.Parallel()

	// The pattern on global.modules.publicDomainTemplate admits exactly one %s. Anything else never
	// went through that schema, and a reservation must not be derived from parts that are not there.
	for _, domainTemplate := range []string{"", "example.com", "%s-%s.example.com", "%s.%s.example.com"} {
		if _, err := ParseNamespace(domainTemplate); err == nil {
			t.Errorf("ParseNamespace(%q) should have failed", domainTemplate)
		}
	}
}

func TestCovers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		domainTemplate string
		host           string
		want           bool
	}{
		{"%s.example.com", "console.example.com", true},
		{"%s.example.com", "shop.example.com", true},
		{"%s.example.com", "my-console.example.com", true},
		{"%s.example.com", "a.example.com", true},
		{"%s.example.com", "*.example.com", true},
		{"%s.example.com", "app.ns.example.com", false},
		{"%s.example.com", "example.com", false},
		{"%s.example.com", "shop.example.org", false},
		{"%s.example.com", "*.ns.example.com", false},
		{"%s.example.com", "-shop.example.com", false},
		{"%s.example.com", "shop-.example.com", false},
		{"kube-%s.company.my", "kube-shop.company.my", true},
		{"kube-%s.company.my", "shop.company.my", false},
		{"kube-%s.company.my", "*.company.my", false},
		{"kube-%s.company.my", "kube-*.company.my", false},
	}

	for _, tc := range cases {
		namespace, err := ParseNamespace(tc.domainTemplate)
		if err != nil {
			t.Fatalf("ParseNamespace(%q): %v", tc.domainTemplate, err)
		}
		if got := namespace.Covers(tc.host); got != tc.want {
			t.Errorf("ParseNamespace(%q).Covers(%q) = %v, want %v", tc.domainTemplate, tc.host, got, tc.want)
		}
	}
}

func TestPatternCoversLeavesOutTheWildcard(t *testing.T) {
	t.Parallel()

	namespace, err := ParseNamespace("%s.example.com")
	if err != nil {
		t.Fatalf("ParseNamespace: %v", err)
	}

	// The wildcard is claimed, but by exact match, and the allowlist the snapshot feeds cannot lift
	// that. Recording it would leave an entry in the record the policies ignore.
	if namespace.PatternCovers(namespace.Wildcard) {
		t.Errorf("PatternCovers(%q) = true, want false", namespace.Wildcard)
	}
	if !namespace.Covers(namespace.Wildcard) {
		t.Errorf("Covers(%q) = false, the reservation does claim it", namespace.Wildcard)
	}
	if !namespace.PatternCovers("shop.example.com") {
		t.Error(`PatternCovers("shop.example.com") = false, want true`)
	}
	if (Namespace{}).PatternCovers("console.example.com") {
		t.Error("the zero Namespace should cover nothing")
	}
}

func TestCoversZeroValueClaimsNothing(t *testing.T) {
	t.Parallel()

	// A namespace that was never parsed must claim nothing rather than everything, the same way an
	// empty hostPattern does in the policies.
	if (Namespace{}).Covers("console.example.com") {
		t.Error("the zero Namespace should cover nothing")
	}
}

func TestNormalizeHost(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"console.example.com":   "console.example.com",
		"Console.Example.COM":   "console.example.com",
		"console.example.com.":  "console.example.com",
		"CONSOLE.EXAMPLE.COM.":  "console.example.com",
		"  console.example.com": "console.example.com",
		"*.EXAMPLE.com":         "*.example.com",
	}

	for host, want := range cases {
		if got := NormalizeHost(host); got != want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestIsHost(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"console.example.com": true,
		"*.example.com":       true,
		"admin":               true,
		"a.b.c.d.example.com": true,
		// What an operator editing grandfatheredHosts by hand can leave behind. Each of these would
		// fail values validation and stop the module that renders Deckhouse from converging, which
		// is why the hook drops them instead of passing them through.
		"Console.Example.COM":       false,
		"console.example.com.":      false,
		"https://admin.example.com": false,
		"admin.example.com:443":     false,
		"-shop.example.com":         false,
		"shop-.example.com":         false,
		"shop..example.com":         false,
		"*.*.example.com":           false,
		"shop.*.example.com":        false,
		"":                          false,
	}

	for host, want := range cases {
		if got := IsHost(host); got != want {
			t.Errorf("IsHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// TestHostPatternIsTheOneTheSchemaValidates keeps the predicate the hook filters a hand-edited record
// with and the pattern values validation applies to the result from drifting apart. A predicate
// looser than the schema lets a value through that fails validation, which is the failure the filter
// exists to prevent.
func TestHostPatternIsTheOneTheSchemaValidates(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("../../../openapi/values.yaml")
	if err != nil {
		t.Fatalf("read the module values schema: %v", err)
	}

	var schema struct {
		Properties struct {
			Internal struct {
				Properties struct {
					ReservedPublicHosts struct {
						Properties struct {
							Hosts struct {
								Items struct {
									Pattern string `json:"pattern"`
								} `json:"items"`
							} `json:"hosts"`
						} `json:"properties"`
					} `json:"reservedPublicHosts"`
				} `json:"properties"`
			} `json:"internal"`
		} `json:"properties"`
	}
	if err := yaml.Unmarshal(content, &schema); err != nil {
		t.Fatalf("parse the module values schema: %v", err)
	}

	got := schema.Properties.Internal.Properties.ReservedPublicHosts.Properties.Hosts.Items.Pattern
	if got != HostPattern {
		t.Errorf("openapi/values.yaml validates %q, HostPattern is %q", got, HostPattern)
	}
}
