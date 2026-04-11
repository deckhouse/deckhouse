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
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/loglabels"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

type addLabelsBuilt struct {
	body     string
	sinkKeys []string
}

func AddLabelsVRL(rule v1alpha1.AddLabelsRule) (string, []string, error) {
	if len(rule.SetLabels) == 0 {
		return "", nil, fmt.Errorf("addLabels: empty setLabels")
	}
	built, err := buildAddLabelsBody(rule.SetLabels)
	if err != nil {
		return "", nil, err
	}
	// If there are no when conditions, return the body without the when block
	if len(rule.When) == 0 {
		return built.body, built.sinkKeys, nil
	}
	exprs := make([]*parser.WhenExpr, 0, len(rule.When))
	for _, w := range rule.When {
		e, err := parser.ParseWhen(w)
		if err != nil {
			return "", nil, err
		}
		exprs = append(exprs, e)
	}
	s, err := wrapAddLabelsWhen(exprs, built.body)
	return s, built.sinkKeys, err
}

func wrapAddLabelsWhen(exprs []*parser.WhenExpr, body string) (string, error) {
	if len(exprs) == 0 {
		return body, nil
	}
	parts := make([]string, 0, len(exprs)+1)
	for i, leaf := range exprs {
		block, err := renderWhenLeaf(i, leaf)
		if err != nil {
			return "", err
		}
		parts = append(parts, block)
	}
	// add final if block to wrap all when blocks
	ifBlock, err := vrl.AddLabelsWhenMultiIf.Render(vrl.Args{
		"cond": whenBoolExprAnd(len(exprs)),
		"body": body,
	})
	if err != nil {
		return "", err
	}
	parts = append(parts, ifBlock)
	return strings.Join(parts, "\n"), nil
}

func renderWhenLeaf(i int, we *parser.WhenExpr) (string, error) {
	args := vrl.Args{
		"i":         i,
		"pathArray": parser.PathSegmentsToVRLArray(we.LeftPathSegs),
		"cmpOp":     string(we.Op),
	}
	switch we.Op {
	case parser.WhenExists, parser.WhenNotExists:
		return vrl.AddLabelsWhenPresenceLeaf.Render(vrl.Args{
			"i":         i,
			"pathArray": parser.PathSegmentsToVRLArray(we.LeftPathSegs),
			"opCmp":     we.Op.PresenceOpCmp(),
		})
	case parser.WhenEQ, parser.WhenNE:
		args["kind"] = "literal"
		args["quotedValue"] = strconv.Quote(we.Value)
		return vrl.AddLabelsWhenLeaf.Render(args)
	case parser.WhenRe, parser.WhenNRe:
		if err := parser.ValidateWhenRegexExpr(we); err != nil {
			return "", err
		}
		op, err := parser.VRLRegexFindComparisonOp(we.Op)
		if err != nil {
			return "", err
		}
		args["kind"] = "regex"
		args["regex"] = escapeRegexLiteral(we.Value)
		args["regexFindOp"] = op
		return vrl.AddLabelsWhenLeaf.Render(args)
	default:
		return "", fmt.Errorf("unsupported when op")
	}
}

func whenBoolExprAnd(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("b_%d", i)
	}
	return strings.Join(parts, " && ")
}

func buildAddLabelsBody(labels map[string]string) (addLabelsBuilt, error) {
	keys := loglabels.SortedMapKeys(labels)
	out := addLabelsBuilt{}
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		segs, err := parser.ParseLabelPath(k)
		if err != nil {
			return addLabelsBuilt{}, fmt.Errorf("addLabels: label key %q: %w", k, err)
		}
		// add sink key for all segments except message
		if len(segs) > 0 && segs[0] != "message" {
			out.sinkKeys = append(out.sinkKeys, strings.TrimPrefix(parser.PathSegmentsToVRLDotPath(segs), "."))
		}
		line, err := buildAddLabelsOneAssignmentFromSegs(segs, labels[k])
		if err != nil {
			return addLabelsBuilt{}, err
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	out.body = strings.Join(lines, "\n")
	return out, nil
}

func buildAddLabelsOneAssignmentFromSegs(segs []string, value string) (string, error) {
	value = strings.TrimSpace(value)
	lhs := parser.PathSegmentsToVRLDotPath(segs)
	// check if value is a mustache template
	if inner, ok := parser.MatchMustachePath(value); ok {
		srcSegs, err := parser.ParseLabelPath(inner)
		if err != nil {
			return "", fmt.Errorf("addLabels template path: %w", err)
		}
		pa := parser.PathSegmentsToVRLArray(srcSegs)
		line, err := vrl.AddLabelsFromPath.Render(vrl.Args{"pathArray": pa, "lhs": lhs})
		return line, err
	}
	// check if value is a mustache without dot prefix
	if _, ok := parser.MatchMustacheGroup(value); ok {
		return "", fmt.Errorf("addLabels: label values must be literals or {{ .path }} references; regex capture group templates are not supported")
	}
	line, err := vrl.AddLabelsAssign.Render(vrl.Args{"lhs": lhs, "rhs": strconv.Quote(value)})
	return line, err
}

func escapeRegexLiteral(s string) string {
	return strings.ReplaceAll(s, `'`, `\'`)
}
