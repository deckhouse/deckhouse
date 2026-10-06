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

package hooks

import (
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const stateLimitNamespacesEntries = `
---
# One entry with a top-level alternation among entries that read the same under both anchorings.
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: team-rules
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: team
  limitNamespaces:
  - team-a|team-b
  - (dev|stage)-.*
  - prod
---
# An entry that is not a valid regular expression.
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: broken-pattern
spec:
  accessLevel: User
  subjects:
  - kind: User
    name: jane
  limitNamespaces:
  - team-(a
---
# namespaceSelector wins over limitNamespaces: the alternation is ignored, the invalid entry is reported as unused.
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: selector-wins
spec:
  accessLevel: User
  subjects:
  - kind: User
    name: john
  namespaceSelector:
    labelSelector:
      matchLabels:
        team: a
  limitNamespaces:
  - a|b
  - b(
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: no-limits
spec:
  accessLevel: User
  subjects:
  - kind: User
    name: bob
`

func limitNamespacesEntryMetricSet(rule, entry, reason string) operation.MetricOperation {
	return operation.MetricOperation{
		Name:   "d8_user_authz_limit_namespaces_entry_needs_rewrite",
		Group:  "d8_user_authz_limit_namespaces_entry_needs_rewrite",
		Action: operation.ActionGaugeSet,
		Value:  ptr.To(1.0),
		Labels: map[string]string{"name": rule, "entry": entry, "reason": reason},
	}
}

var limitNamespacesEntryMetricExpire = operation.MetricOperation{
	Group:  "d8_user_authz_limit_namespaces_entry_needs_rewrite",
	Action: operation.ActionExpireMetrics,
}

var _ = Describe("User Authz hooks :: alert limitNamespaces entries ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"enableMultiTenancy":true,"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "ClusterAuthorizationRule", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("emits only the group expiration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{limitNamespacesEntryMetricExpire}))
		})
	})

	Context("Rules with entries to rewrite", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLimitNamespacesEntries))
			f.RunHook()
		})

		It("emits one metric per entry to rewrite", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(4))
			Expect(m[0]).To(Equal(limitNamespacesEntryMetricExpire))
			Expect(m[1:]).To(ConsistOf(
				limitNamespacesEntryMetricSet("team-rules", "team-a|team-b", "alternation"),
				limitNamespacesEntryMetricSet("broken-pattern", "team-(a", "invalid"),
				limitNamespacesEntryMetricSet("selector-wins", "b(", "invalid-unused"),
			))
		})
	})

	Context("Rules with entries to rewrite, multitenancy is off", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.enableMultiTenancy", false)
			f.BindingContexts.Set(f.KubeStateSet(stateLimitNamespacesEntries))
			f.RunHook()
		})

		It("emits only the group expiration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(Equal([]operation.MetricOperation{limitNamespacesEntryMetricExpire}))
		})
	})

	It("runs on every module run, so that a change of enableMultiTenancy takes effect", func() {
		Expect(f.GoHook.GetConfig().OnBeforeHelm).NotTo(BeNil())
	})

	Context("Module run after multitenancy is turned on", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.enableMultiTenancy", false)
			f.KubeStateSet(stateLimitNamespacesEntries)
			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("emits the metrics without waiting for a rule event", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(4))
			Expect(m[0]).To(Equal(limitNamespacesEntryMetricExpire))
			Expect(m[1:]).To(ConsistOf(
				limitNamespacesEntryMetricSet("team-rules", "team-a|team-b", "alternation"),
				limitNamespacesEntryMetricSet("broken-pattern", "team-(a", "invalid"),
				limitNamespacesEntryMetricSet("selector-wins", "b(", "invalid-unused"),
			))
		})
	})
})

func TestLimitNamespacesEntryReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		entry                string
		namespaceSelectorSet bool
		expected             string
	}{
		{name: "plain name", entry: "team-a", expected: ""},
		{name: "wildcard", entry: "team-.*", expected: ""},
		{name: "top-level alternation", entry: "team-a|team-b", expected: limitNamespacesReasonAlternation},
		{name: "top-level alternation ignored for a namespaceSelector", entry: "team-a|team-b", namespaceSelectorSet: true, expected: ""},
		{name: "optional leading anchor", entry: "^*team-a", expected: limitNamespacesReasonOptionalAnchor},
		{name: "optional leading anchor ignored for a namespaceSelector", entry: "^*team-a", namespaceSelectorSet: true, expected: ""},
		{name: "top-level alternation after an optional leading anchor", entry: "^*team-a|team-b", expected: limitNamespacesReasonAlternation},
		{name: "unclosed group", entry: "team-(a", expected: limitNamespacesReasonInvalid},
		{name: "unclosed group with a namespaceSelector", entry: "team-(a", namespaceSelectorSet: true, expected: limitNamespacesReasonInvalidUnused},
		{name: "lookahead with a namespaceSelector", entry: "(?=team)team-a", namespaceSelectorSet: true, expected: limitNamespacesReasonInvalidUnused},
		{name: "nested repeat with a namespaceSelector", entry: "a**", namespaceSelectorSet: true, expected: limitNamespacesReasonInvalidUnused},
		{name: "unbalanced groups", entry: "a)|(b", expected: limitNamespacesReasonInvalid},
		{name: "unclosed class", entry: "team-[a", expected: limitNamespacesReasonInvalid},
		// the webhook compiles "^team-a\$", whose escaped "$" matches no namespace name
		{name: "trailing backslash", entry: `team-a\`, expected: limitNamespacesReasonAnchoredInvalid},
		{name: "trailing backslash with a namespaceSelector", entry: `team-a\`, namespaceSelectorSet: true, expected: limitNamespacesReasonInvalidUnused},
		{name: "escaped backslash with a namespaceSelector", entry: `team-a\\`, namespaceSelectorSet: true, expected: ""},
		{name: "backreference", entry: `(a)\1`, expected: limitNamespacesReasonInvalid},
		{name: "lookahead", entry: "(?=team)team-a", expected: limitNamespacesReasonInvalid},
		{name: "repeat count over the limit", entry: "a{1001}", expected: limitNamespacesReasonInvalid},
		{name: "unknown posix class", entry: "[[:foo:]]", expected: limitNamespacesReasonInvalid},
		{name: "quote without an end", entry: `\Qteam-a|b`, expected: limitNamespacesReasonAnchoredInvalid},
		{name: "quote without an end ignored for a namespaceSelector", entry: `\Qteam`, namespaceSelectorSet: true, expected: ""},
		// the webhook compiles "^*-dev$", which matches every name that ends with "-dev"
		{name: "leading quantifier", entry: "*-dev", expected: limitNamespacesReasonAnchoredInvalid},
		{name: "leading question mark", entry: "?a", expected: limitNamespacesReasonAnchoredInvalid},
		{name: "leading repeat", entry: "{2}a", expected: limitNamespacesReasonAnchoredInvalid},
		{name: "branches anchored at both ends", entry: "^a$|^b$", expected: ""},
		{name: "branches anchored where the webhook does not anchor them", entry: "a$|^b", expected: ""},
		{name: "branches open to the end on purpose", entry: "^team-a.*|^team-b.*", expected: ""},
		{name: "a later branch with an optional anchor", entry: "a$|^*b", expected: limitNamespacesReasonAlternation},
		{name: "an escaped dollar does not anchor", entry: `a\$|^b`, expected: limitNamespacesReasonAlternation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, limitNamespacesEntryReason(tt.entry, tt.namespaceSelectorSet))
		})
	}
}

// namespaceName matches the names a namespace can have (RFC 1123 labels).
var namespaceName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// FuzzLimitNamespacesEntryReason checks the direction the alert depends on: an entry that is not
// reported matches the same namespace names under both anchorings.
func FuzzLimitNamespacesEntryReason(f *testing.F) {
	for _, seed := range []struct{ entry, name string }{
		{entry: "^*", name: "0"},
		{entry: "team-a|team-b", name: "team-a-dev"},
		{entry: "(team-a|team-b)", name: "team-a"},
		{entry: "team-[]|]", name: "team-a"},
		{entry: "[[:alpha:](]|kube-system", name: "x-kube-system"},
		{entry: `team-[!-\x{5A}-[:alpha:]|]`, name: "team-a"},
		{entry: `\Qteam-a|b\E`, name: "team-a"},
		{entry: "^team-.*$", name: "team-b"},
	} {
		f.Add(seed.entry, seed.name)
	}

	f.Fuzz(func(t *testing.T, entry, name string) {
		if !namespaceName.MatchString(name) || limitNamespacesEntryReason(entry, false) != "" {
			t.Skip()
		}
		concatenated, err := regexp.Compile(anchorByConcatenation(entry))
		if err != nil {
			t.Skip()
		}
		whole := regexp.MustCompile(anchorAsWhole(entry))
		if concatenated.MatchString(name) != whole.MatchString(name) {
			t.Errorf("entry %q is not reported, but the anchorings disagree on %q", entry, name)
		}
	})
}

// anchorByConcatenation anchors an entry the way the authorization webhook does
// (wrapRegex in ee/be/modules/140-user-authz/images/webhook/src/internal/web/hook/handler.go).
func anchorByConcatenation(entry string) string {
	if !strings.HasPrefix(entry, "^") {
		entry = "^" + entry
	}
	if !strings.HasSuffix(entry, "$") {
		entry += "$"
	}
	return entry
}

// anchorAsWhole anchors an entry as one group, so that the anchors apply to every branch.
func anchorAsWhole(entry string) string {
	return "^(?:" + entry + ")$"
}

// namespaceNamesCorpus are the names on which the two anchorings of an entry that is not reported
// must agree. A few of them are not valid namespace names on purpose: they exercise
// the characters the entries put into character classes.
var namespaceNamesCorpus = []string{
	"", "a", "b", "c", "ab", "ax", "xbx", "dev", "prod", "team-", "team-a", "team-b", "team-x",
	"team-a-dev", "x-team-a", "x-team-b", "kube-system", "x-kube-system", "team-|", "team-]", "team-a|b",
}

func TestAlternationChangesMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entry    string
		expected bool
		// witness is a name that the entry anchored by concatenation matches and the entry
		// anchored as a whole does not. It is set only when expected is true.
		witness string
	}{
		{name: "plain name", entry: "team-a"},
		{name: "wildcard", entry: "team-.*"},
		{name: "alternation in a group", entry: "(team-a|team-b)"},
		{name: "alternation in a non-capturing group", entry: "team-(?:a|b)"},
		{name: "alternation in a named group", entry: "team-(?P<suffix>a|b)"},
		{name: "alternation in a group with flags", entry: "team-(?i:a|b)"},
		{name: "pipe in a class", entry: "team-[a|b]"},
		{name: "closing bracket first in a class", entry: "team-[]|]"},
		{name: "closing bracket first in a negated class", entry: "team-[^]|]"},
		{name: "pipe next to a posix class in a class", entry: "team-[[:alpha:]|]"},
		{name: "opening bracket in a class without a posix class", entry: "team-[[|]"},
		{name: "escaped closing bracket in a class", entry: `team-[\]|]`},
		{name: "escaped pipe", entry: `team-a\|b`},
		{name: "escaped parenthesis before a pipe in a group", entry: `(team-\(|b)`},
		{name: "quoted pipe", entry: `\Qteam-a|b\E`},
		{name: "pipe as a hex escape", entry: `team-\x7C`},
		{name: "pipe as an octal escape", entry: `team-\174`},
		{name: "range of braced hex escapes before a posix class", entry: `team-[!-\x{5A}-[:alpha:]|]`},
		{name: "perl class, dash and a posix class", entry: `team-[\d-[:alpha:]|]`},
		{name: "unicode class in a class", entry: `team-[\p{Greek}|]`},
		{name: "flags only", entry: "(?i)team-a"},
		{name: "anchors written into the entry", entry: "^team-a$"},
		{name: "two branches, prefix of the first", entry: "team-a|team-b", expected: true, witness: "team-a-dev"},
		{name: "two branches, suffix of the last", entry: "team-a|team-b", expected: true, witness: "x-team-b"},
		{name: "anchors written into the branches", entry: "^team-a|team-b$", expected: true, witness: "team-a-dev"},
		{name: "middle branch matches anywhere", entry: "a|b|c", expected: true, witness: "xbx"},
		{name: "empty branch matches every name", entry: "prod|", expected: true, witness: "dev"},
		{name: "branches of groups", entry: "(team-a)|(team-b)", expected: true, witness: "team-a-dev"},
		{name: "pipe after a class holding a posix class", entry: "[[:alpha:](]|kube-system", expected: true, witness: "x-kube-system"},
		{name: "pipe after a quoted part", entry: `\Qteam-a\E|team-b`, expected: true, witness: "team-a-dev"},
		{name: "pipe after an escaped parenthesis", entry: `team-\(|team-b`, expected: true, witness: "x-team-b"},
		{name: "pipe after an escaped backslash", entry: `team-\\|team-b`, expected: true, witness: "x-team-b"},
		{name: "pipe after a range that ends in a bracket", entry: "[!-[:alpha:]|]", expected: true, witness: "abc"},
		{name: "pipe after a unicode class", entry: `\pL|team`, expected: true, witness: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			must := require.New(t)
			is := assert.New(t)

			concatenated, err := regexp.Compile(anchorByConcatenation(tt.entry))
			must.NoError(err)
			whole, err := regexp.Compile(anchorAsWhole(tt.entry))
			must.NoError(err)

			is.Equal(tt.expected, alternationChangesMatches(tt.entry))

			if tt.expected {
				is.True(concatenated.MatchString(tt.witness), "anchored by concatenation must match %q", tt.witness)
				is.False(whole.MatchString(tt.witness), "anchored as a whole must not match %q", tt.witness)
				return
			}
			for _, name := range namespaceNamesCorpus {
				is.Equal(concatenated.MatchString(name), whole.MatchString(name), "the anchorings disagree on %q", name)
			}
		})
	}
}

func TestHasOptionalLeadingAnchor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entry    string
		expected bool
		// witness is a name that the entry anchored by concatenation matches and the entry
		// anchored as a whole does not. It is set only when expected is true.
		witness string
	}{
		{name: "no anchor", entry: "team-a"},
		{name: "plain anchor", entry: "^team-a"},
		{name: "anchor repeated at least once", entry: "^+team-a"},
		{name: "anchor followed by an optional anchor", entry: "^^*team-a"},
		{name: "braces that are not a repeat", entry: "^{01}team-a"},
		{name: "optional anchor in a group", entry: "(^)*team-a"},
		{name: "anchor repeated exactly once", entry: "^{1}team-a"},
		{name: "anchor repeated from one", entry: "^{1,3}team-a"},
		{name: "anchor repeated any number of times", entry: "^*team-a", expected: true, witness: "x-team-a"},
		{name: "optional anchor", entry: "^?team-a", expected: true, witness: "x-team-a"},
		{name: "anchor repeated from zero", entry: "^{0,2}team-a", expected: true, witness: "x-team-a"},
		{name: "anchor repeated zero times", entry: "^{0}team-a", expected: true, witness: "x-team-a"},
		{name: "optional anchor alone", entry: "^*", expected: true, witness: "team-a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			must := require.New(t)
			is := assert.New(t)

			concatenated, err := regexp.Compile(anchorByConcatenation(tt.entry))
			must.NoError(err)
			whole, err := regexp.Compile(anchorAsWhole(tt.entry))
			must.NoError(err)

			is.Equal(tt.expected, hasOptionalLeadingAnchor(tt.entry))

			if tt.expected {
				is.True(concatenated.MatchString(tt.witness), "anchored by concatenation must match %q", tt.witness)
				is.False(whole.MatchString(tt.witness), "anchored as a whole must not match %q", tt.witness)
				return
			}
			for _, name := range namespaceNamesCorpus {
				is.Equal(concatenated.MatchString(name), whole.MatchString(name), "the anchorings disagree on %q", name)
			}
		})
	}
}
