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
