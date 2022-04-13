package transform

import (
	"strings"

	// Required to correctly handle omitted fields.
	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/vrl"
)

func CreateLogFilterTransforms(filters []v1alpha1.LogFilter) ([]impl.LogTransform, error) {
	transforms := make([]impl.LogTransform, 0)

	for _, filter := range filters {
		var condition string

		switch filter.Operator {
		case v1alpha1.LogFilterOpExists:
			condition = vrl.LogFilterExistsRule.String(filter.Field)

		case v1alpha1.LogFilterOpDoesNotExist:
			condition = vrl.LogFilterDoesNotExistRule.String(filter.Field)

		case v1alpha1.LogFilterOpIn:
			valuesAsString, err := json.Marshal(filter.Values)
			if err != nil {
				return nil, err
			}
			condition = vrl.LogFilterInRule.String(filter.Field, filter.Field, filter.Field, valuesAsString, valuesAsString, filter.Field)

		case v1alpha1.LogFilterOpNotIn:
			valuesAsString, err := json.Marshal(filter.Values)
			if err != nil {
				return nil, err
			}
			condition = vrl.LogFilterNotInRule.String(filter.Field, filter.Field, filter.Field, valuesAsString, valuesAsString, filter.Field)

		case v1alpha1.LogFilterOpRegex:
			regexps := make([]string, 0)
			for _, regexp := range filter.Values {
				regexps = append(regexps, vrl.LogFilterRegexSingleRule.String(filter.Field, regexp))
			}
			condition = strings.Join(regexps, " || ")

		case v1alpha1.LogFilterOpNotRegex:
			regexps := make([]string, 0)
			for _, regexp := range filter.Values {
				regexps = append(regexps, vrl.LogFilterNotRegexSingleRule.String(filter.Field, regexp))
			}
			condition = vrl.LogFilterNotRegexParentRule.String(filter.Field, filter.Field, strings.Join(regexps, " && "))

		default:
			// no condition was added
			continue
		}

		transforms = append(transforms, &DynamicTransform{
			CommonTransform: CommonTransform{
				Name: "log_filter",
				Type: "filter",
			},
			DynamicArgsMap: map[string]interface{}{
				"condition": condition,
			},
		})
	}

	return transforms, nil
}
