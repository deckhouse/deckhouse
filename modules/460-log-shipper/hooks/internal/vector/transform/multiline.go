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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

const (
	startsWhen = "starts_when"
	endsWhen   = "ends_when"
)

func CreateMultiLineTransforms(multiLineType v1alpha1.MultiLineParserType, multilineCustomConfig v1alpha1.MultilineParserCustom) ([]apis.LogTransform, error) {
	multiLineTransform := &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "multiline",
			Type:   "reduce",
			Inputs: set.New(),
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
		multiLineTransform.DynamicArgsMap[startsWhen] = vrl.GeneralMultilineRule.String()
	case v1alpha1.MultiLineParserBackslash:
		multiLineTransform.DynamicArgsMap[endsWhen] = vrl.BackslashMultilineRule.String()
	case v1alpha1.MultiLineParserLogWithTime:
		multiLineTransform.DynamicArgsMap[startsWhen] = vrl.LogWithTimeMultilineRule.String()
	case v1alpha1.MultiLineParserMultilineJSON:
		multiLineTransform.DynamicArgsMap[startsWhen] = vrl.JSONMultilineRule.String()
	case v1alpha1.MultiLineParserCustom:
		err := processCustomMultiLIneTransform(multilineCustomConfig, multiLineTransform.DynamicArgsMap)
		if err != nil {
			return nil, err
		}
	default:
		return []apis.LogTransform{}, nil
	}

	return []apis.LogTransform{multiLineTransform}, nil
}

func processCustomMultiLIneTransform(multilineCustomConfig v1alpha1.MultilineParserCustom, argMap map[string]interface{}) error {
	if multilineCustomConfig.EndsWhen != nil {
		endsWhenRule, err := processMultilineRegex(multilineCustomConfig.EndsWhen)
		if err != nil {
			return err
		}
		argMap[endsWhen] = endsWhenRule
	}
	if multilineCustomConfig.StartsWhen != nil {
		endsWhenRule, err := processMultilineRegex(multilineCustomConfig.StartsWhen)
		if err != nil {
			return err
		}
		argMap[startsWhen] = *endsWhenRule
	}
	return nil
}

func processMultilineRegex(parserRegex *v1alpha1.ParserRegex) (*string, error) {
	if parserRegex == nil {
		return nil, fmt.Errorf("no regex provided")
	}
	if parserRegex.NotRegex != nil && parserRegex.Regex != nil {
		return nil, fmt.Errorf("must be set one of regex or notRegex")
	}

	var (
		resultRegexRule vrl.Rule
		multilineRegex  string
	)

	switch {
	case parserRegex.NotRegex != nil:
		resultRegexRule = vrl.NotRegexMultilineRule
		multilineRegex = *parserRegex.NotRegex
	case parserRegex.Regex != nil:
		resultRegexRule = vrl.RegexMultilineRule
		multilineRegex = *parserRegex.Regex
	default:
		return nil, fmt.Errorf("regex or notRegex should be provided")
	}

	resultRegex, err := resultRegexRule.Render(vrl.Args{"multiline": multilineRegex})
	if err != nil {
		return nil, fmt.Errorf("can't render regex: %v", err)
	}
	return &resultRegex, nil
}
