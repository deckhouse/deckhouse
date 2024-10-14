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
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

func CreateParseDataTransforms() *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "parse_json",
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"source":        vrl.ParseJSONRule.String(),
			"drop_on_abort": false,
		},
	}
}

type mutateFilter func(*v1alpha1.Filter, *vrl.Rule)

func CreateLogFilterTransforms(filters []v1alpha1.Filter) ([]apis.LogTransform, error) {
	transforms, err := createFilterTransform("log_filter", filters, func(filter *v1alpha1.Filter, _ *vrl.Rule) {
		// parsed_data is a key for parsed json data from a message, we use it to quickly filter inputs
		// "filter_field" -> "parsed_data.filter_field", "" -> "parsed_data"
		if filter.Field == "" {
			filter.Field = "parsed_data"
		} else {
			filter.Field = strings.Join([]string{"parsed_data", filter.Field}, ".")
		}
	})
	if err != nil {
		return nil, err
	}
	if len(transforms) > 0 {
		transforms = append([]apis.LogTransform{CreateParseDataTransforms()}, transforms...)
	}
	return transforms, nil
}

func CreateLabelFilterTransforms(filters []v1alpha1.Filter) ([]apis.LogTransform, error) {
	return createFilterTransform("label_filter", filters, nil)
}

func createFilterTransform(name string, filters []v1alpha1.Filter, mutate mutateFilter) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)

	for _, filter := range filters {
		rule := getRuleOutOfFilter(&filter)
		if rule == "" {
			continue
		}

		if mutate != nil {
			mutate(&filter, &rule)
		}

		condition, err := rule.Render(vrl.Args{"filter": filter})
		if err != nil {
			return nil, err
		}

		transforms = append(transforms, &DynamicTransform{
			CommonTransform: CommonTransform{
				Name:   name,
				Type:   "filter",
				Inputs: set.New(),
			},
			DynamicArgsMap: map[string]interface{}{
				"condition": condition,
			},
		})
	}
	return transforms, nil
}

func getRuleOutOfFilter(filter *v1alpha1.Filter) vrl.Rule {
	var rule vrl.Rule

	switch filter.Operator {
	case v1alpha1.FilterOpExists:
		rule = vrl.FilterExistsRule

	case v1alpha1.FilterOpDoesNotExist:
		rule = vrl.FilterDoesNotExistRule

	case v1alpha1.FilterOpIn:
		rule = vrl.FilterInRule

	case v1alpha1.FilterOpNotIn:
		rule = vrl.FilterNotInRule

	case v1alpha1.FilterOpRegex:
		rule = vrl.FilterRegexRule
		filter.Values = matchFullRegexp(filter.Values)

	case v1alpha1.FilterOpNotRegex:
		rule = vrl.FilterNotRegexRule
		filter.Values = matchFullRegexp(filter.Values)

	default:
		// no condition was added
	}
	return rule
}

// make regexps to match full strings
// "d8-.*" -> "^d8-$"
func matchFullRegexp(regexes []interface{}) []interface{} {
	for index, raw := range regexes {
		regex, ok := raw.(string)
		if !ok {
			// malformed, should be string by the custom resource definition
			continue
		}
		if !strings.HasPrefix(regex, "^") {
			regex = "^" + regex
		}

		if !strings.HasSuffix(regex, "$") {
			regex += "$"
		}

		regexes[index] = regex
	}
	return regexes
}
