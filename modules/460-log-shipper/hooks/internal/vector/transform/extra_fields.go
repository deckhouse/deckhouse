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
)

// Copied these regexes from another place. Remove them after refactoring.
var (
	vectorArrayTemplate   = regexp.MustCompile(`^[a-zA-Z0-9_\\\.\-]+\[\d+\]$`)
	validMustacheTemplate = regexp.MustCompile(`^\{\{\ ([a-zA-Z0-9][a-zA-Z0-9\[\]_\\\-\.]+)\ \}\}$`)
)

// ExtraFieldTransform converts templated labels to values.
//
// TODO(nabokihms): Honestly, I do not know exactly how this function works.
//   Only required for Elasticsearch sinks.
//   It definitely deserves refactoring. My assumption is that it generates VRL rules from extra labels.
//   Example:
//     label_name: {{ values.app }} -> .label_name = .values.app
func ExtraFieldTransform(extraFields map[string]string) *DynamicTransform {

	var dataField string

	tmpFields := make([]string, 0, len(extraFields))
	keys := make([]string, 0, len(extraFields))
	for key := range extraFields {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, k := range keys {
		if validMustacheTemplate.MatchString(extraFields[k]) {
			dataField = validMustacheTemplate.FindStringSubmatch(extraFields[k])[1]
			if dataField == "parsed_data" {
				tmpFields = append(tmpFields, fmt.Sprintf(" if exists(.parsed_data) { .%s=.parsed_data } \n", k))
			} else {
				tmpDataFieldParts := strings.Split(dataField, ".")
				dataFieldParts := make([]string, 0)
				i := 0
				for i < len(tmpDataFieldParts) {
					if tmpDataFieldParts[i][len(tmpDataFieldParts[i])-1] == '\\' && i+1 <= len(tmpDataFieldParts) {
						buf := tmpDataFieldParts[i]
						iter := i + 1
						for iter < len(tmpDataFieldParts) {
							if tmpDataFieldParts[iter][len(tmpDataFieldParts[iter])-1] != '\\' {
								buf = buf + "." + tmpDataFieldParts[iter]
								break
							}
							buf = buf + "." + tmpDataFieldParts[iter]
							iter++
						}
						dataFieldParts = append(dataFieldParts, buf)
						i = iter + 1
					} else {
						dataFieldParts = append(dataFieldParts, tmpDataFieldParts[i])
						i++
					}
				}
				for i := range dataFieldParts {
					if strings.Contains(dataFieldParts[i], "-") || strings.Contains(dataFieldParts[i], "\\") {
						if vectorArrayTemplate.MatchString(dataFieldParts[i]) {
							arrayVarParts := strings.Split(dataFieldParts[i], "[")
							dataFieldParts[i] = fmt.Sprintf("\"%s\"[%s", strings.ReplaceAll(arrayVarParts[0], "\\", ""), arrayVarParts[1])
						} else {
							dataFieldParts[i] = fmt.Sprintf("\"%s\"", strings.ReplaceAll(dataFieldParts[i], "\\", ""))
						}
					}
				}
				tmpFields = append(tmpFields, fmt.Sprintf(" if exists(.parsed_data.%s) { .%s=.parsed_data.%s } \n", strings.Join(dataFieldParts, "."), k, strings.Join(dataFieldParts, ".")))
			}
		} else {
			tmpFields = append(tmpFields, fmt.Sprintf(" .%s=\"%s\" \n", k, extraFields[k]))
		}
	}

	extraFieldsTransform := DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "extra_fields",
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        strings.Join(tmpFields, ""),
			"drop_on_abort": false,
		},
	}

	return &extraFieldsTransform
}
