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

package transform

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

// Copied these regexes from another place. Remove them after refactoring.
var (
	vectorArrayTemplate   = regexp.MustCompile(`^[a-zA-Z0-9_\\\.\-]+\[\d+\]$`)
	validMustacheTemplate = regexp.MustCompile(`^\{\{\ ([a-zA-Z0-9][a-zA-Z0-9\[\]_\\\-\.]+)\ \}\}$`)
)

const (
	parsedDataField = "parsed_data"
)

// ExtraFieldTransform converts templated labels to values.
// It generates valid VRL remaps from key-value pairs
//
//	Only required for Elasticsearch sinks.
//	Example:
//	  label_name: {{ values.app }} -> .label_name = .values.app
func ExtraFieldTransform(extraFields map[string]string) *DynamicTransform {
	tmpFields := make([]string, 0)
	keys := mapKeys(extraFields)

	for _, k := range keys {
		tmpFields = append(tmpFields, processExtraFieldKey(k, extraFields[k]))
	}

	extraFieldsTransform := DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "extra_fields",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.Combine(vrl.ParseJSONRule, vrl.Rule(strings.Join(tmpFields, ""))).String(),
			"drop_on_abort": false,
		},
	}

	return &extraFieldsTransform
}

// mapKeys returns sorted keys of map
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

// processExtraFieldKey processes key-value pairs to valid vrls
// used for extra deild remap transformation
//
// example with template in value:
// aaa: {{ pay-load[0].a }} -> if exists(.parsed_data."pay-load"[0].a) { .aaa=.parsed_data."pay-load"[0].a }
//
// example with template and parsed_data in value:
// abc: {{ parsed_data }} -> if exists(.parsed_data) { .abc=.parsed_data }
//
// example with plain string in value:
// aba: bbb -> .aba="bbb"
func processExtraFieldKey(key, value string) string {
	if key == "" {
		return ""
	}

	key = escapeVectorString(key)

	if !validMustacheTemplate.MatchString(value) {
		return fmt.Sprintf(" .%s=%q \n", key, value)
	}

	// From regex lib docs:
	//   If 'Submatch' is present, the return value is a slice identifying the
	//   successive submatches of the expression. Submatches are matches of
	//   parenthesized subexpressions (also known as capturing groups) within the
	//   regular expression, numbered from left to right in order of opening
	//   parenthesis. Submatch 0 is the match of the entire expression, submatch 1 is
	//   the match of the first parenthesized subexpression, and so on.
	//
	// for example, for string `{{ parsed_data.asas }}` there would be
	// two submatches for expression from 'validMustacheTemplate' variable:
	// `{{ parsed_data.asas }}` and `parsed_data.asas`
	//
	dataField := validMustacheTemplate.FindStringSubmatch(value)[1]
	if dataField == parsedDataField {
		return fmt.Sprintf(" if exists(.%s) { .%s=.%s } \n", parsedDataField, key, parsedDataField)
	}

	dataField = generateDataField(dataField)
	return fmt.Sprintf(" if exists(.%s.%s) { .%s=.%s.%s } \n", parsedDataField, dataField, key, parsedDataField, dataField)
}

// generateDataField escapes field for valid vrl. In detail,
// this func splits field by dots (`.`), then it iterates over
// splitted slices and determines fields with
// 'processDataFieldWithEscape' function, for example:
// `test.pay\.lo\.ad.hel\.lo.world` -> [`test`, `pay\.lo\.ad`, `hel\.lo`, `world`]
// and then it escapes every field and concatenates them back
func generateDataField(dataField string) string {
	tmpDataFieldParts := strings.Split(dataField, ".")
	if tmpDataFieldParts[0] == parsedDataField {
		tmpDataFieldParts = tmpDataFieldParts[1:]
	}

	dataFieldParts := make([]string, 0)
	i := 0
	for i < len(tmpDataFieldParts) {
		buf, iter := processDataFieldWithEscape(i, tmpDataFieldParts)
		dataFieldParts = append(dataFieldParts, buf)
		i = iter + 1
	}

	for i := range dataFieldParts {
		dataFieldParts[i] = escapeVectorString(dataFieldParts[i])
	}
	return strings.Join(dataFieldParts, ".")
}

// processDataFieldWithEscape retrieves full field from datafield parts.
// this func would iterate over tmpDataFieldParts from i to n (n > i), where n is index,
// which corresponds to first string after ith string, that DOESN'T ends with `\\`
// for example:
// tmpDataFieldParts := []string{"test", "pay\", "lo\", "ad", "hel\", "lo", "world"}
// i := 0 -> `test`
// i := 1 -> `pay\.lo\.ad`
// i := 4 -> `hel\.lo`
// i := 6 -> `world`
func processDataFieldWithEscape(i int, tmpDataFieldParts []string) (string, int) {
	buf := tmpDataFieldParts[i]
	if buf[len(buf)-1] != '\\' || i+1 > len(tmpDataFieldParts) {
		return buf, i
	}

	iter := i + 1
	for iter < len(tmpDataFieldParts) {
		iterBuf := tmpDataFieldParts[iter]
		if iterBuf[len(iterBuf)-1] != '\\' {
			buf = buf + "." + iterBuf
			break
		}
		buf = buf + "." + iterBuf
		iter++
	}
	return buf, iter
}

// escapeVectorString func escapes "-" and "." in labels and removes "\" in string
// example: `pay\.lo[3]` -> `"pay.lo"[3]`
// example: `pay-load[0]` -> `"pay-load"[0]`
func escapeVectorString(s string) string {
	if strings.Contains(s, "-") || strings.Contains(s, "\\") {
		if vectorArrayTemplate.MatchString(s) {
			arrayVarParts := strings.Split(s, "[")
			return fmt.Sprintf("\"%s\"[%s", strings.ReplaceAll(arrayVarParts[0], "\\", ""), arrayVarParts[1])
		}
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(s, "\\", ""))
	}
	return s
}
