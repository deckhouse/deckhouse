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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/loglabels"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transformation/parser"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

var parseMessageUnaryByFormat = map[v1alpha1.SourceFormat]string{
	v1alpha1.FormatKlog:   "parse_klog(.message)",
	v1alpha1.FormatCLF:    "parse_common_log(.message)",
	v1alpha1.FormatSysLog: "parse_syslog(.message)",
	v1alpha1.FormatLogfmt: "parse_logfmt(.message)",
}

var parseMessageStringTargetFieldPattern = regexp.MustCompile(`^[a-zA-Z0-9_\\\.\-]+$`)

func jsonDepthClause(depth int) string {
	if depth == 0 {
		return ""
	}
	return fmt.Sprintf(", max_depth: %d", depth)
}

func GenerateParseMessageVRL(spec v1alpha1.ParseMessageSpec) (string, error) {
	targetLabel := strings.TrimSpace(spec.TargetLabel)
	if targetLabel == "" {
		targetLabel = v1alpha1.DefaultParseMessageTargetLabel
	}
	root := targetLabel == "."
	var segs []string
	var err error
	if !root {
		segs, err = parser.ParseLabelPath(targetLabel)
		if err != nil {
			return "", err
		}
	}
	switch spec.SourceFormat {
	case v1alpha1.FormatJSON:
		pe := fmt.Sprintf("parse_json(.message%s)", jsonDepthClause(spec.JSON.Depth))
		return parseRule(vrl.ParseMessageDest, segs, root, vrl.Args{"parseExpr": pe})
	case v1alpha1.FormatString:
		return parseMessageStringFormat(segs, root, spec)
	default:
		parseExpr, ok := parseMessageUnaryByFormat[spec.SourceFormat]
		if !ok {
			return "", fmt.Errorf("parseMessage: unknown sourceFormat %q", spec.SourceFormat)
		}
		return parseRule(vrl.ParseMessageDest, segs, root, vrl.Args{"parseExpr": parseExpr})
	}
}

func parseRule(rule vrl.Rule, segs []string, root bool, extra vrl.Args) (string, error) {
	args := vrl.Args{"mergeRoot": root}
	if !root {
		args["pathArray"] = parser.PathSegmentsToVRLArray(segs)
	}
	for k, v := range extra {
		args[k] = v
	}
	return rule.Render(args)
}

func parseMessageStringFormat(segs []string, root bool, spec v1alpha1.ParseMessageSpec) (string, error) {
	s := spec.String
	// if targetField is set, use it as the target field
	if s.TargetField != "" {
		if !parseMessageStringTargetFieldPattern.MatchString(s.TargetField) {
			return "", fmt.Errorf("parseMessage string: invalid targetField %q", s.TargetField)
		}
		return parseRule(vrl.ParseMessageString, segs, root, vrl.Args{"targetField": s.TargetField})
	}
	if s.Regex == "" {
		return "", fmt.Errorf("parseMessage string: regex and setLabels required when targetField is empty")
	}
	if len(s.SetLabels) == 0 {
		return "", fmt.Errorf("parseMessage string: setLabels required with regex")
	}
	if _, err := regexp.Compile(s.Regex); err != nil {
		return "", fmt.Errorf("parseMessage string regex: %w", err)
	}
	return parseMessageStringRegex(segs, root, s)
}

func parseMessageStringRegex(segs []string, root bool, s v1alpha1.SourceFormatStringSpec) (string, error) {
	keys := loglabels.SortedMapKeys(s.SetLabels)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		val, err := parseMessageRegexLabelValueToVRL(s.SetLabels[k])
		if err != nil {
			return "", err
		}
		line, err := vrl.ParseMessageRegexStringOut.Render(vrl.Args{
			"key":   k,
			"value": val,
		})
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	return parseRule(vrl.ParseMessageRegexString, segs, root, vrl.Args{
		"regex":    escapeRegexLiteral(s.Regex),
		"outLines": strings.Join(lines, "\n"),
	})
}

func parseMessageRegexLabelValueToVRL(v string) (string, error) {
	v = strings.TrimSpace(v)
	// if value is a mustache group, return the capture string
	if group, ok := parser.MatchMustacheGroup(v); ok {
		return vrl.RegexCaptureString.Render(vrl.Args{"name": group})
	}
	// if value is not a mustache group, quote it
	return strconv.Quote(v), nil
}
