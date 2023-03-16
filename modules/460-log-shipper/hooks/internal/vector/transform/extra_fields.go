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

func processExtraFieldKey(key string, value string) string {
	key = escapeVectorString(key)
	if validMustacheTemplate.MatchString(value) {
		dataField := validMustacheTemplate.FindStringSubmatch(value)[1]
		if dataField == "parsed_data" {
			return fmt.Sprintf(" if exists(.parsed_data) { .%s=.parsed_data } \n", key)
		}
		dataField = combineDataFieldParts(generateDataFieldParts(dataField))
		return fmt.Sprintf(" if exists(.parsed_data.%s) { .%s=.parsed_data.%s } \n", dataField, key, dataField)
	}
	return fmt.Sprintf(" .%s=\"%s\" \n", key, value)

}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0)
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

func generateDataFieldParts(dataField string) []string {
	tmpDataFieldParts := strings.Split(dataField, ".")
	dataFieldParts := make([]string, 0)
	i := 0
	for i < len(tmpDataFieldParts) {
		buf, iter := processDataFieldWithEscape(i, tmpDataFieldParts)
		dataFieldParts = append(dataFieldParts, buf)
		i = iter + 1
	}
	return dataFieldParts
}

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

func combineDataFieldParts(dataFieldParts []string) string {
	for i := range dataFieldParts {
		dataFieldParts[i] = escapeVectorString(dataFieldParts[i])
	}
	return strings.Join(dataFieldParts, ".")
}

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
