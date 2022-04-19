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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/vrl"
)

func CreateMultiLineTransforms(multiLineType v1alpha1.MultiLineParserType) []impl.LogTransform {
	multiLineTransform := DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "multiline",
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
		multiLineTransform.DynamicArgsMap["starts_when"] = vrl.GeneralMultilineRule.String()
	case v1alpha1.MultiLineParserBackslash:
		multiLineTransform.DynamicArgsMap["ends_when"] = vrl.BackslashMultilineRule.String()
	case v1alpha1.MultiLineParserLogWithTime:
		multiLineTransform.DynamicArgsMap["starts_when"] = vrl.LogWithTimeMultilineRule.String()
	case v1alpha1.MultiLineParserMultilineJSON:
		multiLineTransform.DynamicArgsMap["starts_when"] = vrl.JSONMultilineRule.String()
	default:
		return []impl.LogTransform{}
	}

	return []impl.LogTransform{&multiLineTransform}
}
