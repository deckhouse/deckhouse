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

func BuildModes(tms []v1alpha1.Transform) ([]apis.LogTransform, error) {
	transforms := make([]apis.LogTransform, 0)
	var module module
	for _, tm := range tms {
		switch tm.Action {
		case "replaceDot":
			fmt.Println("Labels ", len(tm.Labels))
			if len(tm.Labels) == 0 {
				tm.Labels = []string{"pod_labels"}
			}
			module = swapDot{labels: tm.Labels}
		case "wrapNotJson":
			if len(tm.Labels) == 0 {
				tm.Labels = []string{"message"}
			}
			module = wrapNotJson{labels: tm.Labels}
		case "delete":
			if len(tm.Labels) == 0 {
				return nil, fmt.Errorf("Transform operation delete haven`t label")
			}
			module = delete{labels: tm.Labels}
		default:
			return nil, fmt.Errorf("TransformMod: action %s not found", tm.Action)
		}
		lofTransform := module.getTransform()
		transforms = append(transforms, lofTransform)
	}
	return transforms, nil
}

type swapDot struct {
	labels []string
}

func (swap swapDot) getTransform() apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_key_rename_%s", strings.Join(swap.labels, "_"))
	labels := checkFixDotPrefix(swap.labels)
	fmt.Println(labels, swap.labels)
	for _, l := range labels {
		vrl = fmt.Sprintf("%sif exists(%s) {\n%s = map_keys(object!(%s), recursive: true) -> |key| { replace(key, \".\", \"_\")})\n}",
			vrl, l, l, l)
	}
	return NewTransformation(name, vrl)
}

type wrapNotJson struct {
	labels []string
}

func (wrap wrapNotJson) getTransform() apis.LogTransform {
	var vrl string
	name := fmt.Sprintf("tf_warp_not_json_%s", strings.Join(wrap.labels, "_"))
	labels := checkFixDotPrefix(wrap.labels)
	for _, l := range labels {
		vrl = fmt.Sprintf("%s%s = parse_json(%s) ?? { \"text\": %s }\n", vrl, l, l, l)
	}
	return NewTransformation(name, vrl)
}

type delete struct {
	labels []string
}

func (d delete) getTransform() apis.LogTransform {
	name := fmt.Sprintf("tf_delete_%s", strings.Join(d.labels, "_"))
	labels := checkFixDotPrefix(d.labels)
	strLabels := strings.Join(labels, ", ")
	vrl := fmt.Sprintf("del(%s)\n", strLabels)
	return NewTransformation(name, vrl)
}

func NewTransformation(name, vrl string) *DynamicTransform {
	return &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   name,
			Type:   "remap",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]any{
			"source":        vrl,
			"drop_on_abort": false,
		},
	}
}

// add Dot in label prefix  if not exist
func checkFixDotPrefix(labels []string) []string {
	for i, l := range labels {
		if !strings.HasPrefix(l, ".") {
			labels[i] = fmt.Sprintf(".%s", l)
		}
	}
	return labels
}
