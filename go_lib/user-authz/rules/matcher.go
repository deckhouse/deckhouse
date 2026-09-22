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

package rules

import (
	"regexp"
	"strings"
	"sync"
)

// Matcher answers whether a namespace is covered by one limitNamespaces pattern.
type Matcher struct {
	// entry is the pattern as the rule wrote it. Everything that has to reason ABOUT the pattern
	// reads this and not the anchored form: the anchoring is an implementation detail of matching,
	// and code that parsed it back was where the anchoring bugs lived.
	entry string
	// pattern is the anchored form, ^(?:entry)$, and exists only to be compiled.
	pattern string
	// literal is set when the entry contains no regular-expression metacharacters: most rules
	// name their namespaces outright, and a string comparison is both faster and free of the
	// compiled *regexp.Regexp that costs kilobytes per rule.
	literal string
	re      *regexp.Regexp
}

// Entry returns the pattern as the rule wrote it.
func (m Matcher) Entry() string { return m.entry }

// Matches reports whether the namespace is covered.
func (m Matcher) Matches(namespace string) bool {
	if m.re == nil {
		return namespace == m.literal
	}
	return m.re.MatchString(namespace)
}

// MatchesEverything reports whether the pattern covers every namespace name. Such a pattern means
// the rule limits nothing, and only the system-namespace gate remains.
//
// It reads the entry, not the anchored form. Both spellings a rule can use for "anything" are
// listed: the bare quantifier and the one where the author wrote the anchors themselves.
func (m Matcher) MatchesEverything() bool {
	switch m.entry {
	case ".*", ".+", "^.*$", "^.+$":
		return true
	}
	return false
}

// WrapRegex anchors a limitNamespaces entry: the patterns are matched against the whole namespace
// name, so "team" must not cover "team-2".
//
// The group is not optional. Alternation binds looser than anything else in a regular expression,
// so anchoring "team-.*|kube-system" by concatenation gives (^team-.*)|(kube-system$): the second
// branch is anchored only at the end and matches any namespace whose name ENDS in "kube-system",
// "attacker-kube-system" among them. Nobody writing `a|b` in limitNamespaces means that.
//
// Every entry is wrapped, whether or not it looks like it needs it. Deciding required scanning the
// pattern for an alternation that separates whole branches, and a scanner that has to know when a
// "|" is nested, quoted or inside a character class is a small regular-expression parser with the
// same bug surface as the thing it guards: a POSIX class name, "[[:alpha:](]|kube-system", ended
// the class at the "]" of ":alpha:", counted the "(" as a group and left the alternation
// unanchored - the very hole the anchoring exists to close. Wrapping also needed the entry's own
// anchors trimmed, which mistook an escaped "b\$" for an anchor and produced "^(?:a|b\)$", a
// pattern that does not compile at all.
//
// Wrapping unconditionally has neither problem, and it costs nothing: ^(?:p)$ and the older
// concatenated form accept exactly the same namespace names for every pattern that compiled -
// TestWrapRegex_UnconditionalGroupMatchesTheOldAnchoring pins that on the whole corpus.
func WrapRegex(pattern string) string {
	return "^(?:" + pattern + ")$"
}

// regexMetacharacters are the characters that make a limitNamespaces entry a regular expression
// rather than a namespace name. Namespace names are DNS labels, so none of them appears in a literal.
const regexMetacharacters = `\.+*?()|[]{}^$`

// compileCache interns compiled patterns: the same pattern appears in many rules of a cluster
// (every team rule ends in ".*", every namespace name is bound by several rules), and a rebuild of
// the directory must not recompile what it compiled last time.
type compileCache struct {
	mu       sync.Mutex
	matchers map[string]Matcher
}

func newCompileCache() *compileCache {
	return &compileCache{matchers: make(map[string]Matcher)}
}

// compile returns the Matcher of a limitNamespaces entry, compiling it once per pattern.
func (c *compileCache) compile(entry string) (Matcher, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.matchers[entry]; ok {
		return m, nil
	}

	// Keyed on the entry, and the literal fast path reads the entry too. Both used to work off the
	// anchored form and had to strip the anchors back off, which is the parsing that went wrong.
	// An entry that writes its own anchors - "^team-a$" - therefore takes the compiled path now
	// rather than the string comparison; it is a rare way to write a rule and the answer is the
	// same either way.
	m := Matcher{entry: entry, pattern: WrapRegex(entry)}
	if !strings.ContainsAny(entry, regexMetacharacters) {
		m.literal = entry
	} else {
		re, err := regexp.Compile(m.pattern)
		if err != nil {
			return Matcher{}, err
		}
		m.re = re
	}
	c.matchers[entry] = m
	return m, nil
}

// evict drops the patterns that no rule uses any more, so a cluster that rewrote all its rules does
// not keep the old expressions alive forever.
func (c *compileCache) evict(inUse map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for pattern := range c.matchers {
		if _, ok := inUse[pattern]; !ok {
			delete(c.matchers, pattern)
		}
	}
}
