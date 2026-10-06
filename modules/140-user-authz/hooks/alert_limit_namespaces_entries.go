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

// Finds the limitNamespaces entries of ClusterAuthorizationRules that have to be rewritten and
// publishes one metric per entry for the D8UserAuthzLimitNamespacesEntryNeedsRewrite and
// D8UserAuthzLimitNamespacesEntryInvalid alerts.
//
// The authorization webhook anchors an entry by adding "^" in front and "$" at the end unless the
// entry already starts with "^" or ends with "$", so "a|b" becomes "^a|b$": its first branch
// matches a prefix of the namespace name, its last branch a suffix, and a middle branch any part.
// Anchored as a whole, "^(?:a|b)$", the same entry matches only "a" and "b". The two anchorings
// differ for an entry with a "|" outside every group, character class, escape and \Q...\E quote,
// and for an entry that starts with a "^" a quantifier makes optional, such as "^*a".
//
// An entry that Go regexp cannot compile the way the webhook anchors it is reported as invalid: the
// webhook leaves such an entry out, the rule still limits its subjects, and they lose the namespaces
// only that entry covers. On a rule with a namespaceSelector the webhook does not read the entries at
// all, so there such an entry is reported as invalid-unused, for a rewrite: it has no effect, but the
// stricter validation refuses to create or change a rule with an entry that is structurally broken
// or uses a construct RE2 does not have, whatever its namespaceSelector. So is an entry of such a
// rule that ends with a backslash, which the webhook anchoring compiles as an escaped "$". An entry
// that compiles the way the webhook anchors it but not anchored as a whole, such as "*-dev" or an
// unterminated \Q, is reported for a rewrite: anchored as a whole, it matches nothing.
//
// The hook reads the entries only, never the namespaces, so its cost does not grow with the number
// of namespaces, and the filter keeps nothing but the entries to report.

package hooks

import (
	"context"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode/utf8"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	limitNamespacesEntriesSnapshot = "limit_namespaces_entries"

	// limitNamespacesEntryMetric has one time series per entry to rewrite, labelled with the rule
	// name, the entry and the reason: alternation, optional-anchor, anchored-invalid, invalid or
	// invalid-unused.
	limitNamespacesEntryMetric = "d8_user_authz_limit_namespaces_entry_needs_rewrite"

	// limitNamespacesReasonAlternation marks an entry with a top-level "|".
	limitNamespacesReasonAlternation = "alternation"
	// limitNamespacesReasonOptionalAnchor marks an entry that starts with a "^" a quantifier makes
	// optional.
	limitNamespacesReasonOptionalAnchor = "optional-anchor"
	// limitNamespacesReasonAnchoredInvalid marks an entry that compiles the way the webhook anchors
	// it, but not anchored as a whole.
	limitNamespacesReasonAnchoredInvalid = "anchored-invalid"
	// limitNamespacesReasonInvalid marks an entry that is not a valid regular expression the way the
	// webhook anchors it, on a rule without a namespaceSelector.
	limitNamespacesReasonInvalid = "invalid"
	// limitNamespacesReasonInvalidUnused marks such an entry on a rule with a namespaceSelector,
	// which the webhook does not read.
	limitNamespacesReasonInvalidUnused = "invalid-unused"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue(limitNamespacesEntriesSnapshot),
	// The module run re-evaluates the entries when enableMultiTenancy changes.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       limitNamespacesEntriesSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: applyLimitNamespacesEntriesFilter,
		},
	},
}, handleLimitNamespacesEntries)

type limitNamespacesEntry struct {
	Entry  string `json:"entry"`
	Reason string `json:"reason"`
}

type ruleLimitNamespacesEntries struct {
	Name    string                 `json:"name"`
	Entries []limitNamespacesEntry `json:"entries"`
}

// applyLimitNamespacesEntriesFilter keeps a rule only when it has entries to report, and only those
// entries.
func applyLimitNamespacesEntriesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	rawEntries, found, err := unstructured.NestedSlice(obj.Object, "spec", "limitNamespaces")
	if err != nil {
		return nil, fmt.Errorf("read spec.limitNamespaces of ClusterAuthorizationRule %q: %w", obj.GetName(), err)
	}
	if !found {
		return nil, nil
	}

	// The webhook uses limitNamespaces only when the rule has no namespaceSelector.
	selector, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "namespaceSelector")
	if err != nil {
		return nil, fmt.Errorf("read spec.namespaceSelector of ClusterAuthorizationRule %q: %w", obj.GetName(), err)
	}
	namespaceSelectorSet := found && selector != nil

	entries := make([]limitNamespacesEntry, 0)
	for _, raw := range rawEntries {
		entry, ok := raw.(string)
		if !ok {
			continue
		}
		if reason := limitNamespacesEntryReason(entry, namespaceSelectorSet); reason != "" {
			entries = append(entries, limitNamespacesEntry{Entry: entry, Reason: reason})
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}

	return ruleLimitNamespacesEntries{Name: obj.GetName(), Entries: entries}, nil
}

// handleLimitNamespacesEntries publishes the entries to rewrite. With enableMultiTenancy off the
// authorization webhook is not deployed and limitNamespaces has no effect, so nothing is published.
func handleLimitNamespacesEntries(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(limitNamespacesEntryMetric)

	if !input.Values.Get("userAuthz.enableMultiTenancy").Bool() {
		return nil
	}

	snapshot := input.Snapshots.Get(limitNamespacesEntriesSnapshot)
	for rule, err := range sdkobjectpatch.SnapshotIter[ruleLimitNamespacesEntries](snapshot) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", limitNamespacesEntriesSnapshot, err)
		}
		for _, entry := range rule.Entries {
			input.MetricsCollector.Set(limitNamespacesEntryMetric, 1,
				map[string]string{"name": rule.Name, "entry": entry.Entry, "reason": entry.Reason},
				metrics.WithGroup(limitNamespacesEntryMetric))
		}
	}
	return nil
}

// limitNamespacesEntryReason returns why the entry has to be rewritten, or "" when it does not.
//
// An invalid entry is reported whatever the namespaceSelector, as invalid-unused when the rule has
// one, since the webhook then does not use the entry at all. So is an entry of
// such a rule that ends with a backslash, which the stricter validation refuses as well. An entry
// that the two anchorings read differently is reported only when the rule has no namespaceSelector,
// because otherwise the entry is not used.
func limitNamespacesEntryReason(entry string, namespaceSelectorSet bool) string {
	switch {
	case !compiles(anchorLikeTheWebhook(entry)):
		if namespaceSelectorSet {
			return limitNamespacesReasonInvalidUnused
		}
		return limitNamespacesReasonInvalid
	case namespaceSelectorSet:
		if hasTrailingBackslash(entry) {
			return limitNamespacesReasonInvalidUnused
		}
		return ""
	case !compiles(anchorLimitNamespacesEntry(entry)):
		return limitNamespacesReasonAnchoredInvalid
	case alternationChangesMatches(entry):
		return limitNamespacesReasonAlternation
	case hasOptionalLeadingAnchor(entry):
		return limitNamespacesReasonOptionalAnchor
	}
	return ""
}

// hasTrailingBackslash reports whether the entry ends with a backslash that escapes nothing, the way
// the stricter validation checks it: one backslash at the end, not two.
func hasTrailingBackslash(entry string) bool {
	return strings.HasSuffix(entry, `\`) && !strings.HasSuffix(entry, `\\`)
}

func compiles(pattern string) bool {
	_, err := regexp.Compile(pattern)
	return err == nil
}

// anchorLikeTheWebhook anchors the entry the way the authorization webhook does: "^" in front and "$"
// at the end, unless the entry already starts or ends with it.
func anchorLikeTheWebhook(entry string) string {
	if !strings.HasPrefix(entry, "^") {
		entry = "^" + entry
	}
	if !strings.HasSuffix(entry, "$") {
		entry += "$"
	}
	return entry
}

// anchorLimitNamespacesEntry anchors the entry as a whole. An entry can compile the way the webhook
// anchors it and not in this form: a leading quantifier has nothing to repeat, and an unterminated
// \Q quotes the closing parenthesis too.
func anchorLimitNamespacesEntry(entry string) string {
	return "^(?:" + entry + ")$"
}

// alternationChangesMatches reports whether a pattern that Go regexp compiles has top-level branches
// the two anchorings read differently. The webhook anchors only the start of the first branch and the
// end of the last one, so a later branch that does not start with "^" or ".*", or an earlier one that
// does not end with "$" or ".*", matches more of a name than it does anchored as a whole.
func alternationChangesMatches(pattern string) bool {
	branches := topLevelBranches(pattern)
	for i, branch := range branches {
		if i > 0 && !startsAnchored(branch) {
			return true
		}
		if i < len(branches)-1 && !endsAnchored(branch) {
			return true
		}
	}
	return false
}

// startsAnchored reports whether a branch matches only from the start of the name, or anything at
// the start, whether or not "^" is added in front of it.
func startsAnchored(branch string) bool {
	return strings.HasPrefix(branch, ".*") || strings.HasPrefix(branch, "^") && !hasOptionalLeadingAnchor(branch)
}

// endsAnchored reports whether a branch matches only up to the end of the name, or anything at the
// end, whether or not "$" is added after it.
func endsAnchored(branch string) bool {
	escaped := func(suffix string) bool { return strings.HasSuffix(branch, `\`+suffix) }
	return strings.HasSuffix(branch, "$") && !escaped("$") || strings.HasSuffix(branch, ".*") && !escaped(".*")
}

// topLevelBranches splits a pattern that Go regexp compiles at every "|" outside every group,
// character class, escape and \Q...\E quote. It reads the pattern the way the regexp/syntax parser
// does and gives no meaningful answer for a pattern that does not compile.
func topLevelBranches(pattern string) []string {
	var branches []string
	start, depth := 0, 0
	for t := pattern; t != ""; {
		offset := len(pattern) - len(t)
		switch t[0] {
		case '\\':
			if strings.HasPrefix(t, `\Q`) {
				// Everything up to \E, or to the end, is literal.
				_, t, _ = strings.Cut(t[2:], `\E`)
				continue
			}
			if rest, ok := cutClassEscape(t); ok {
				t = rest
				continue
			}
			t = cutEscape(t)
		case '[':
			t = cutCharacterClass(t)
		case '(':
			depth++
			t = t[1:]
		case ')':
			depth--
			t = t[1:]
		case '|':
			if depth == 0 {
				branches = append(branches, pattern[start:offset])
				start = offset + 1
			}
			t = t[1:]
		default:
			t = t[1:]
		}
	}
	return append(branches, pattern[start:])
}

// cutCharacterClass returns what follows the character class that t starts with. It follows
// parseClass of regexp/syntax: a "]" right after "[" or "[^" is a literal, a POSIX class, a Unicode
// class or a Perl class escape is one element, and any other element is a character or an escape,
// optionally followed by "-" and the end of a range.
func cutCharacterClass(t string) string {
	t = strings.TrimPrefix(t[1:], "^")
	for first := true; t != "" && (t[0] != ']' || first); first = false {
		if rest, ok := cutPOSIXClass(t); ok {
			t = rest
			continue
		}
		if rest, ok := cutClassEscape(t); ok {
			t = rest
			continue
		}
		t = cutClassChar(t)
		if len(t) >= 2 && t[0] == '-' && t[1] != ']' {
			t = cutClassChar(t[1:])
		}
	}
	if t == "" {
		return ""
	}
	return t[1:]
}

// cutPOSIXClass cuts a leading "[:name:]". Like regexp/syntax, it takes "[:" for the start of a
// POSIX class name only when ":]" follows somewhere later.
func cutPOSIXClass(t string) (string, bool) {
	if len(t) <= 2 || !strings.HasPrefix(t, "[:") {
		return t, false
	}
	end := strings.Index(t[2:], ":]")
	if end < 0 {
		return t, false
	}
	return t[2+end+2:], true
}

// cutClassEscape cuts a leading Unicode class (\pN, \p{Name}, \PN, \P{Name}) or Perl class escape
// (\d, \D, \s, \S, \w, \W).
func cutClassEscape(t string) (string, bool) {
	if len(t) < 2 || t[0] != '\\' {
		return t, false
	}
	switch t[1] {
	case 'p', 'P':
		if len(t) < 3 {
			return t, false
		}
		if t[2] != '{' {
			_, size := utf8.DecodeRuneInString(t[2:])
			return t[2+size:], true
		}
		end := strings.IndexByte(t, '}')
		if end < 0 {
			return t, false
		}
		return t[end+1:], true
	case 'd', 'D', 's', 'S', 'w', 'W':
		return t[2:], true
	}
	return t, false
}

// cutClassChar cuts one character of a class: a rune or an escape.
func cutClassChar(t string) string {
	if t[0] == '\\' {
		return cutEscape(t)
	}
	_, size := utf8.DecodeRuneInString(t)
	return t[size:]
}

// cutEscape cuts a leading escape the way parseEscape of regexp/syntax reads it: an octal escape
// takes up to three digits, \x takes two hex digits or a braced number, and any other escape is two
// bytes long.
func cutEscape(t string) string {
	if len(t) < 2 {
		return ""
	}
	rest := t[2:]
	switch c := t[1]; {
	case c >= '0' && c <= '7':
		for n := 0; n < 2 && rest != "" && rest[0] >= '0' && rest[0] <= '7'; n++ {
			rest = rest[1:]
		}
	case c == 'x':
		if !strings.HasPrefix(rest, "{") {
			return rest[min(2, len(rest)):]
		}
		if end := strings.IndexByte(rest, '}'); end >= 0 {
			return rest[end+1:]
		}
	}
	return rest
}

// hasOptionalLeadingAnchor reports whether a pattern that Go regexp compiles starts with a "^" that
// a quantifier makes optional, as in "^*a", "^?a" or "^{0,1}a". The webhook adds no "^" to an entry
// that starts with one, so such an entry is not anchored at the start.
func hasOptionalLeadingAnchor(pattern string) bool {
	if !strings.HasPrefix(pattern, "^") {
		return false
	}
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return false
	}
	first := re
	if re.Op == syntax.OpConcat && len(re.Sub) > 0 {
		first = re.Sub[0]
	}
	switch first.Op {
	case syntax.OpStar, syntax.OpQuest:
		return true
	case syntax.OpRepeat:
		return first.Min == 0
	}
	return false
}
