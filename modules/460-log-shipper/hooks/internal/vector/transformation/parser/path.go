/*
Copyright 2021 Flant JSC

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

package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var vrlBarePathSegmentRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var mustachePathRe = regexp.MustCompile(`^\{\{\s*(\.[a-zA-Z0-9\[\]_\\\-\."']+)\s*\}\}$`)
var mustacheGroupRe = regexp.MustCompile(`^\s*\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}\s*$`)

// isVRLBarePathSegment returns true for simple identifiers like `foo`, `my_key`.
// Segments containing special characters (e.g. `foo-bar`) require quoting in VRL.
func isVRLBarePathSegment(s string) bool {
	return vrlBarePathSegmentRe.MatchString(s)
}

// ParseLabelPath splits a dot-prefixed label path into segments.
// example: `.foo.bar`          → ["foo", "bar"]
// example: `.msg."a-b".level`  → ["msg", "a-b", "level"]
// example: `.k."a\"b"`         → ["k", `a"b`]
func ParseLabelPath(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	if !strings.HasPrefix(path, ".") {
		return nil, fmt.Errorf("path must start with a dot")
	}
	return parsePathSegments(path)
}

func parsePathSegments(s string) ([]string, error) {
	var segs []string
	for len(s) > 0 {
		s = strings.TrimLeft(s, ".")
		if len(s) == 0 {
			break
		}
		seg, rest, err := nextPathSegment(s)
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
		s = rest
	}
	if len(segs) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	return segs, nil
}

func nextPathSegment(s string) (string, string, error) {
	switch {
	case strings.HasPrefix(s, `"`):
		end, seg, err := readQuotedSegment(s, '"')
		if err != nil {
			return "", "", err
		}
		return seg, s[end:], nil
	case strings.HasPrefix(s, "'"):
		end, seg, err := readQuotedSegment(s, '\'')
		if err != nil {
			return "", "", err
		}
		return seg, s[end:], nil
	default:
		i := strings.IndexByte(s, '.')
		if i < 0 {
			i = len(s)
		}
		// i == 0 means s starts with '.', which is a double-dot — no valid segment name
		if i == 0 {
			return "", "", fmt.Errorf("invalid path segment near %q", s)
		}
		return s[:i], s[i:], nil
	}
}

// readQuotedSegment reads a quoted segment starting at s[0]==q.
// Returns consumed length, unescaped content, and error.
// example: `"foo-bar".rest`  → (9, "foo-bar", nil)
// example: `"a\"b"`          → (6, `a"b`, nil)
func readQuotedSegment(s string, q byte) (int, string, error) {
	if len(s) < 2 {
		return 0, "", fmt.Errorf("unclosed quoted segment")
	}
	pos := 1
	for {
		idx := strings.IndexByte(s[pos:], q)
		if idx < 0 {
			return 0, "", fmt.Errorf("unclosed quoted segment")
		}
		idx += pos
		if s[idx-1] == '\\' {
			pos = idx + 1
			continue
		}
		return idx + 1, unescape(s[1:idx], q), nil
	}
}

// unescape replaces \q → q and \\ → \ in a raw segment content.
// example: unescape(`a\"b`, '"') → `a"b`
// example: unescape(`no_esc`, '"') → `no_esc`
func unescape(s string, q byte) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	s = strings.ReplaceAll(s, string([]byte{'\\', q}), string(q))
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// PathSegmentsToVRLArray formats segments as a VRL array literal.
func PathSegmentsToVRLArray(segments []string) string {
	quoted := make([]string, len(segments))
	for i, seg := range segments {
		quoted[i] = strconv.Quote(seg)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// PathSegmentsToVRLDotPath formats segments as a VRL dot-notation path.
// Bare identifiers stay unquoted, special characters get quoted.
// example: ["foo", "bar"]   → `.foo.bar`
// example: ["msg", "a-b"]  → `.msg."a-b"`
func PathSegmentsToVRLDotPath(segments []string) string {
	parts := make([]string, len(segments))
	for i, seg := range segments {
		if isVRLBarePathSegment(seg) {
			parts[i] = "." + seg
			continue
		}
		parts[i] = "." + strconv.Quote(seg)
	}
	return strings.Join(parts, "")
}

// MapLabelPaths parses each dot-prefixed label path and applies f to the segments slice.
func MapLabelPaths(labels []string, f func([]string) string) ([]string, error) {
	out := make([]string, 0, len(labels))
	for _, pl := range labels {
		segs, err := ParseLabelPath(pl)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", pl, err)
		}
		out = append(out, f(segs))
	}
	return out, nil
}

// SinkKeysFromVRLPaths returns each path as a VRL dot path without the leading dot (for loglabels drop prefixes; same rule as addLabels sink keys).
func SinkKeysFromVRLPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		segs, err := ParseLabelPath(p)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", p, err)
		}
		k := strings.TrimPrefix(PathSegmentsToVRLDotPath(segs), ".")
		if k != "" {
			out = append(out, k)
		}
	}
	return out, nil
}

// MatchMustachePath extracts the inner path from a mustache template.
// example: `{{ .foo.bar }}`  → (".foo.bar", true)
// example: `plain`           → ("", false)
func MatchMustachePath(s string) (string, bool) {
	m := mustachePathRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", false
	}
	return m[1], true
}

// MatchMustacheGroup extracts a regex capture group name from a mustache template.
// example: `{{ grp }}`  → ("grp", true)
// example: `literal`    → ("", false)
func MatchMustacheGroup(s string) (string, bool) {
	m := mustacheGroupRe.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return m[1], true
}
