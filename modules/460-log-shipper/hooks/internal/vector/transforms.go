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

package vector

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

func cleanUpTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source": ` if exists(.pod_labels."controller-revision-hash") {
    del(.pod_labels."controller-revision-hash")
 }
  if exists(.pod_labels."pod-template-hash") {
   del(.pod_labels."pod-template-hash")
 }
 if exists(.kubernetes) {
   del(.kubernetes)
 }
 if exists(.file) {
   del(.file)
 }
`,
			"drop_on_abort": false,
		},
	}
}

func cleanDataTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        `if exists(.parsed_data) { del(.parsed_data) }`,
			"drop_on_abort": false,
		},
	}
}

// jsonParseTransform is a default logstash & elasticsearch json parser transform
func jsonParseTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source": ` structured, err1 = parse_json(.message)
 if err1 == null {
   .parsed_data = structured
 }
`,
			"drop_on_abort": false,
		},
	}
}

// deDotTransform is a default logstash & elasticsearch dedot transform
// Related issue https://github.com/timberio/vector/issues/3588
func deDotTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "lua",
		},
		DynamicArgsMap: map[string]interface{}{
			"version": "2",
			"hooks": map[string]interface{}{
				"process": "process",
			},
			"source": `
function process(event, emit)
	if event.log.pod_labels == nil then
		return
	end
	dedot(event.log.pod_labels)
	emit(event)
end
function dedot(map)
	if map == nil then
		return
	end
	local new_map = {}
	local changed_keys = {}
	for k, v in pairs(map) do
		local dedotted = string.gsub(k, "%.", "_")
		if dedotted ~= k then
			new_map[dedotted] = v
			changed_keys[k] = true
		end
	end
	for k in pairs(changed_keys) do
		map[k] = nil
	end
	for k, v in pairs(new_map) do
		map[k] = v
	end
end
`,
		},
	}
}

func dataStreamTransform() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        `."@timestamp" = del(.timestamp)`,
			"drop_on_abort": false,
		},
	}
}

func extraFieldTransform(extraFields map[string]string) *DynamicTransform {

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
			Type: "remap",
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        strings.Join(tmpFields, ""),
			"drop_on_abort": false,
		},
	}

	return &extraFieldsTransform
}

// CreateDefaultTransforms creates predefined transform instruction for every log source
func CreateDefaultTransforms(dest v1alpha1.ClusterLogDestination) []impl.LogTransform {
	// data stream elasticsearch transform

	// default cleanup transform
	transforms := []impl.LogTransform{
		cleanUpTransform(),
		jsonParseTransform(),
	}

	// Adding specific storage transforms
	if dest.Spec.Type == DestElasticsearch || dest.Spec.Type == DestLogstash {
		transforms = append(transforms, deDotTransform())

		if len(dest.Spec.ExtraLabels) > 0 {
			transforms = append(transforms, extraFieldTransform(dest.Spec.ExtraLabels))
		}
	}

	if dest.Spec.Type == DestElasticsearch && dest.Spec.Elasticsearch.DataStreamEnabled {
		transforms = append(transforms, dataStreamTransform())
	}

	return transforms
}

func CreateDefaultCleanUpTransforms(dest v1alpha1.ClusterLogDestination) []impl.LogTransform {
	transforms := make([]impl.LogTransform, 0)
	if dest.Spec.Type == DestElasticsearch || dest.Spec.Type == DestLogstash {
		transforms = append(transforms, cleanDataTransform())
	}
	return transforms
}

func CreateMultiLineTransforms(multiLineType v1alpha1.MultiLineParserType) []impl.LogTransform {
	// default multiline transform
	var multiLineTransform DynamicTransform = DynamicTransform{
		CommonTransform: CommonTransform{
			Type: "reduce",
		},
		DynamicArgsMap: map[string]interface{}{
			"group_by": []string{
				"file",
				"stream",
			},
			"merge_strategies": map[string]string{
				"message": "concat",
			},
		},
	}

	switch multiLineType {
	case v1alpha1.MultiLineParserGeneral:
		multiLineTransform.DynamicArgsMap["starts_when"] = " if exists(.message) { if length(.message) > 0 { matched, err = match(.message, r'^[^\\s\\t]'); if err != null { false; } else { matched; }; } else { false; }; } else { false; } "
	case v1alpha1.MultiLineParserBackslash:
		multiLineTransform.DynamicArgsMap["ends_when"] = " matched, err = match(.message, r'[^\\\\]$'); if err != null { false; } else { matched; } "
	case v1alpha1.MultiLineParserLogWithTime:
		multiLineTransform.DynamicArgsMap["starts_when"] = " matched, err = match(.message, r'^\\[?((((19|20)([2468][048]|[13579][26]|0[48])|2000)-02-29|((19|20)[0-9]{2}-(0[4678]|1[02])-(0[1-9]|[12][0-9]|30)|(19|20)[0-9]{2}-(0[1359]|11)-(0[1-9]|[12][0-9]|3[01])|(19|20)[0-9]{2}-02-(0[1-9]|1[0-9]|2[0-8])))\\s([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9])|20\\d\\d-(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)-(0[1-9]|[1-2][0-9]|3[01])\\s([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9])|(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\\s+\\d{1,2}\\s+([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9])|(?:(\\d{4}-\\d{2}-\\d{2})T(\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?))(Z|[\\+-]\\d{2}:\\d{2})?|\\p{L}{2}\\s\\d{1,2}\\s\\p{L}{3}\\s\\d{4}\\s([01][0-9]|2[0-3]):([012345][0-9]):([012345][0-9]))'); if err != null { false; } else { matched; } "
	case v1alpha1.MultiLineParserMultilineJSON:
		multiLineTransform.DynamicArgsMap["starts_when"] = " matched, err = match(.message, r'^\\{'); if err != null { false; } else { matched; } "
	default:
		return []impl.LogTransform{}
	}

	return []impl.LogTransform{&multiLineTransform}
}

func CreateTransformsFromFilter(filters []v1alpha1.LogFilter) (transforms []impl.LogTransform, err error) {
	transforms = make([]impl.LogTransform, 0)

	for _, filter := range filters {
		switch filter.Operator {
		case v1alpha1.LogFilterOpExists:
			transforms = append(transforms, &DynamicTransform{
				CommonTransform: CommonTransform{
					Type: "filter",
				},
				DynamicArgsMap: map[string]interface{}{
					"condition": fmt.Sprintf("exists(.parsed_data.%s)", filter.Field),
				},
			})
		case v1alpha1.LogFilterOpDoesNotExist:
			transforms = append(transforms, &DynamicTransform{
				CommonTransform: CommonTransform{
					Type: "filter",
				},
				DynamicArgsMap: map[string]interface{}{
					"condition": fmt.Sprintf("!exists(.parsed_data.%s)", filter.Field),
				},
			})
		case v1alpha1.LogFilterOpIn:
			valuesAsString, err := json.Marshal(filter.Values)
			if err != nil {
				return nil, err
			}
			transforms = append(transforms, &DynamicTransform{
				CommonTransform: CommonTransform{
					Type: "filter",
				},
				DynamicArgsMap: map[string]interface{}{
					"condition": fmt.Sprintf(`if is_boolean(.parsed_data.%s) || is_float(.parsed_data.%s) { data, err = to_string(.parsed_data.%s); if err != null { false; } else { includes(%s, data); }; } else { includes(%s, .parsed_data.%s); }`, filter.Field, filter.Field, filter.Field, valuesAsString, valuesAsString, filter.Field),
				},
			})
		case v1alpha1.LogFilterOpNotIn:
			valuesAsString, err := json.Marshal(filter.Values)
			if err != nil {
				return nil, err
			}
			transforms = append(transforms, &DynamicTransform{
				CommonTransform: CommonTransform{
					Type: "filter",
				},
				DynamicArgsMap: map[string]interface{}{
					"condition": fmt.Sprintf(`if is_boolean(.parsed_data.%s) || is_float(.parsed_data.%s) { data, err = to_string(.parsed_data.%s); if err != null { true; } else { !includes(%s, data); }; } else { !includes(%s, .parsed_data.%s); }`, filter.Field, filter.Field, filter.Field, valuesAsString, valuesAsString, filter.Field),
				},
			})
		case v1alpha1.LogFilterOpRegex:
			regexps := make([]string, 0)
			for _, regexp := range filter.Values {
				regexps = append(regexps, fmt.Sprintf("match!(.parsed_data.%s, r'%s')", filter.Field, regexp))
			}
			transforms = append(transforms, &DynamicTransform{
				CommonTransform: CommonTransform{
					Type: "filter",
				},
				DynamicArgsMap: map[string]interface{}{
					"condition": strings.Join(regexps, " || "),
				},
			})
		case v1alpha1.LogFilterOpNotRegex:
			regexps := make([]string, 0)
			for _, regexp := range filter.Values {
				regexps = append(regexps, fmt.Sprintf(`{ matched, err = match(.parsed_data.%s, r'%s')
 if err != null {
 true
 } else {
 !matched
 }}`, filter.Field, regexp))
			}
			transforms = append(transforms, &DynamicTransform{
				CommonTransform: CommonTransform{
					Type: "filter",
				},
				DynamicArgsMap: map[string]interface{}{
					"condition": fmt.Sprintf(`if exists(.parsed_data.%s) && is_string(.parsed_data.%s)
 {
 %s
 } else {
 true
 }`, filter.Field, filter.Field, strings.Join(regexps, " && ")),
				},
			})
		default:
			continue
		}
	}

	return
}

func BuildTransformsFromMapSlice(inputName string, trans []impl.LogTransform) ([]impl.LogTransform, error) {
	prevInput := inputName

	for i, trm := range trans {
		trm.SetName(fmt.Sprintf("d8_tf_%s_%d", inputName, i))
		trm.SetInputs([]string{prevInput})
		prevInput = trm.GetName()
		trans[i] = trm
	}

	return trans, nil
}

type CommonTransform struct {
	Name   string   `json:"-"`
	Type   string   `json:"type"`
	Inputs []string `json:"inputs"`
}

func (cs *CommonTransform) GetName() string {
	return cs.Name
}

func (cs *CommonTransform) SetName(name string) {
	cs.Name = name
}

func (cs *CommonTransform) SetInputs(inp []string) {
	cs.Inputs = inp
}

func (cs *CommonTransform) GetInputs() []string {
	return cs.Inputs
}

type DynamicTransform struct {
	CommonTransform

	DynamicArgsMap map[string]interface{} `json:"-"`
}

func (t DynamicTransform) MarshalJSON() ([]byte, error) {

	type dt DynamicTransform // prevent recursion
	b, _ := json.Marshal(dt(t))

	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)

	for k, v := range t.DynamicArgsMap {
		b, _ = json.Marshal(v)
		m[k] = b
	}

	return json.Marshal(m)
}

func (t DynamicTransform) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var transformMap map[string]interface{}
	err := unmarshal(&transformMap)
	if err != nil {
		return err
	}

	tstr, ok := transformMap["type"].(string)
	if !ok {
		return errors.New("type required and have to be string")
	}

	inp, ok := transformMap["inputs"].([]string)
	if !ok {
		inp = make([]string, 0)
	}

	delete(transformMap, "inputs")
	delete(transformMap, "type")

	newtr := DynamicTransform{
		CommonTransform: CommonTransform{
			Type:   tstr,
			Inputs: inp,
		},
		DynamicArgsMap: transformMap,
	}
	t = newtr

	return nil
}
