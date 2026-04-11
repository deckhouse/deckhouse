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

package transformation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

var replaceValueMustacheNamedGroupRe = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

func ReplaceValueVRL(spec v1alpha1.ReplaceValueSpec) (string, error) {
	if spec.Source == "" {
		return "", fmt.Errorf("replaceValue: source is empty")
	}
	if _, err := regexp.Compile(spec.Source); err != nil {
		return "", fmt.Errorf("replaceValue: source regex: %w", err)
	}
	if len(spec.Labels) == 0 {
		return "", fmt.Errorf("replaceValue: labels is empty")
	}
	srcEsc := escapeRegexLiteral(spec.Source)
	pathArrays, err := parser.MapLabelPaths(spec.Labels, parser.PathSegmentsToVRLArray)
	if err != nil {
		return "", fmt.Errorf("replaceValue: %w", err)
	}
	blocks := make([]string, 0, len(pathArrays))
	for _, pa := range pathArrays {
		block, err := replaceValueOnePath(pa, srcEsc, spec.Target)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, strings.TrimSpace(block))
	}
	return strings.Join(blocks, "\n"), nil
}

func replaceValueOnePath(pathArray, srcEsc, target string) (string, error) {
	args := vrl.Args{
		"pathArray":   pathArray,
		"sourceRegex": srcEsc,
	}
	// if target is not a mustache named group, use a literal replacement
	if !replaceValueMustacheNamedGroupRe.MatchString(target) {
		args["useNamedGroups"] = false
		args["targetQuoted"] = strconv.Quote(target)
		return vrl.ReplaceValueRule.Render(args)
	}
	replExpr, err := replaceValueBuildExprFromMustacheTarget(target)
	if err != nil {
		return "", err
	}
	args["useNamedGroups"] = true
	args["replacementExpr"] = replExpr
	return vrl.ReplaceValueRule.Render(args)
}

// replaceValueBuildExprFromMustacheTarget turns replaceValue target into a VRL expression: literals are quoted,
// each {{ name }} becomes string!(parsed.name) for a named capture from parse_regex (same convention as parseMessage string-regex).
// example: `{{ grp }}`               → string!(parsed.grp)
// example: `pre-{{ grp }}-post`      → "pre-" + string!(parsed.grp) + "-post"
// example: `fixed`                   → "fixed"
func replaceValueBuildExprFromMustacheTarget(target string) (string, error) {
	pos := 0
	var parts []string
	for {
		loc := replaceValueMustacheNamedGroupRe.FindStringSubmatchIndex(target[pos:])
		if loc == nil {
			// no more mustache named groups
			if pos < len(target) {
				parts = append(parts, strconv.Quote(target[pos:]))
			}
			break
		}
		matchStart := pos + loc[0]
		// add literal before the match
		if matchStart > pos {
			parts = append(parts, strconv.Quote(target[pos:matchStart]))
		}
		group := target[pos+loc[2] : pos+loc[3]]
		// add named capture group
		expr, err := vrl.RegexCaptureString.Render(vrl.Args{"name": group})
		if err != nil {
			return "", err
		}
		parts = append(parts, expr)
		pos += loc[1] // move past the match
	}
	return strings.Join(parts, " + "), nil
}
