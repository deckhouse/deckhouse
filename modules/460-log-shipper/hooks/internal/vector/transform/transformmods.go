package transform

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

type module interface {
	getTransform() apis.LogTransform
}

func BuildModes(tms []v1alpha1.TransformMod) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)
	var module module
	for _, tm := range tms {
		switch tm.Action {
		case "replaceDot":
			module = replaceDot{}
		case "fixNestedJson":
			if tm.Label == "" {
				tm.Label = "message"
			}
			module = fixNestedJson{label: tm.Label}
		case "del":
			if tm.Label != "" {
				module = del{label: tm.Label}
			}
		default:
			return nil, fmt.Errorf("TransformMod: action %s not found", tm.Action)
		}
		lofTransform := module.getTransform()
		transforms = append(transforms, lofTransform)
	}
	return transforms, nil
}

type replaceDot struct{}

func (r replaceDot) getTransform() apis.LogTransform {
	return DeDotTransform()
}

type fixNestedJson struct {
	label string
}

func (fix fixNestedJson) getTransform() apis.LogTransform {
	label := checkFixDotPrefix(fix.label)
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   fmt.Sprintf("parse_json_label_%s", fix.label),
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]any{
			"source":        fmt.Sprintf("%s = parse_json(%s) ?? { \"text\": %s }\n", label, label, label),
			"drop_on_abort": false,
		},
	}
}

type del struct {
	label string
}

func (d del) getTransform() apis.LogTransform {
	label := checkFixDotPrefix(d.label)
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   fmt.Sprintf("drop_label_%s", d.label),
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]any{
			"source":        fmt.Sprintf("del(%s)\n", label),
			"drop_on_abort": false,
		},
	}
}

// add Dot in label prefix  if not exist
func checkFixDotPrefix(l string) string {
	if !strings.HasPrefix(l, ".") {
		l = fmt.Sprintf(".%s", l)
	}
	return l
}
