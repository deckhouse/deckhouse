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
	// pattern is the anchored form of the entry as written by the user, ^...$.
	pattern string
	// literal is set when the pattern contains no regular-expression metacharacters: most rules
	// name their namespaces outright, and a string comparison is both faster and free of the
	// compiled *regexp.Regexp that costs kilobytes per rule.
	literal string
	re      *regexp.Regexp
}

// Pattern returns the anchored pattern.
func (m Matcher) Pattern() string { return m.pattern }

// Matches reports whether the namespace is covered.
func (m Matcher) Matches(namespace string) bool {
	if m.re == nil {
		return namespace == m.literal
	}
	return m.re.MatchString(namespace)
}

// MatchesEverything reports whether the pattern covers every namespace name. Such a pattern means
// the rule limits nothing, and only the system-namespace gate remains.
func (m Matcher) MatchesEverything() bool {
	switch m.pattern {
	case "^.*$", "^.+$":
		return true
	}
	return false
}

// WrapRegex anchors a limitNamespaces entry: the patterns are matched against the whole namespace
// name, so "team" must not cover "team-2".
//
// An alternation needs a group around it. Alternation binds looser than anything else in a regular
// expression, so anchoring "team-.*|kube-system" by concatenation gives (^team-.*)|(kube-system$):
// the second branch is anchored only at the end, and matches any namespace whose name ENDS in
// "kube-system" - "attacker-kube-system" among them. Nobody writing `a|b` in limitNamespaces means
// that. A non-capturing group anchors every branch.
//
// Everything without a top-level alternation is anchored by concatenation exactly as before, which
// keeps the compiled pattern of an ordinary rule byte-identical: the literal fast path below reads
// the pattern back, and so does MatchesEverything.
func WrapRegex(pattern string) string {
	if hasTopLevelAlternation(pattern) {
		inner := strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")
		return "^(?:" + inner + ")$"
	}
	if !strings.HasPrefix(pattern, "^") {
		pattern = "^" + pattern
	}
	if !strings.HasSuffix(pattern, "$") {
		pattern += "$"
	}
	return pattern
}

// hasTopLevelAlternation reports whether the pattern contains a "|" that separates whole branches,
// as opposed to one nested in a group or inside a character class where it is a literal.
func hasTopLevelAlternation(pattern string) bool {
	depth := 0
	inClass := false
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++ // whatever follows is a literal, including "|", "[" and "("
		case '[':
			if !inClass {
				inClass = true
			}
		case ']':
			inClass = false
		case '(':
			if !inClass {
				depth++
			}
		case ')':
			if !inClass && depth > 0 {
				depth--
			}
		case '|':
			if !inClass && depth == 0 {
				return true
			}
		}
	}
	return false
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
	pattern := WrapRegex(entry)

	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.matchers[pattern]; ok {
		return m, nil
	}

	m := Matcher{pattern: pattern}
	inner := strings.TrimSuffix(strings.TrimPrefix(pattern, "^"), "$")
	if !strings.ContainsAny(inner, regexMetacharacters) {
		m.literal = inner
	} else {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return Matcher{}, err
		}
		m.re = re
	}
	c.matchers[pattern] = m
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
