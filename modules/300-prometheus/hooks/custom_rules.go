package hooks

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CustomRule struct {
	Name   string
	Groups []interface{}
}

func filterCustomRule(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cr := new(CustomRule)
	cr.Name = obj.GetName()

	groupsRaw, ok, err := unstructured.NestedSlice(obj.Object, "spec", "groups")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("no groups field")
	}

	for _, gr := range groupsRaw {
		group := gr.(interface{})
		cr.Groups = append(cr.Groups, group)
	}

	return cr, nil
}

type InternalRule struct {
	Name string
}

func filterInternalRule(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ir := new(InternalRule)
	ir.Name = obj.GetName()
	return ir, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/custom_prometheus_rules",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "rules",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "CustomPrometheusRules",
			FilterFunc: filterCustomRule, // jqFilter: '{"name": .metadata.name, "groups": .spec.groups}'
		},
		{
			Name:       "internal_rules",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"module":     "prometheus",
					"heritage":   "deckhouse",
					"app":        "prometheus",
					"prometheus": "main",
					"component":  "rules",
				},
			},
			FilterFunc: filterInternalRule, //  jqFilter: '.metadata.name'
		},
	},
}, customRulesHandler)

func customRulesHandler(input *go_hook.HookInput) error {
	tmpMap := make(map[string]bool)

	rulesSnap := input.Snapshots["rules"]

	for _, ruleF := range rulesSnap {
		rule := ruleF.(*CustomRule)
		internalRule := createPrometheusRule(rule.Name, rule.Groups)
		err := input.ObjectPatcher.CreateOrUpdateObject(&internalRule, "")
		if err != nil {
			return err
		}

		tmpMap[internalRule.GetName()] = true
	}

	internalRulesSnap := input.Snapshots["internal_rules"]

	// delete absent prometheus rules
	for _, sn := range internalRulesSnap {
		inRule := sn.(*InternalRule)
		if _, ok := tmpMap[inRule.Name]; !ok {
			err := input.ObjectPatcher.DeleteObject("monitoring.coreos.com/v1", "PrometheusRule", "d8-monitoring", inRule.Name, "")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createPrometheusRule(name string, groups []interface{}) unstructured.Unstructured {
	// apiVersion: monitoring.coreos.com/v1
	// kind: PrometheusRule
	// metadata:
	//  name: d8-custom-${name}
	//  namespace: d8-monitoring
	//  labels:
	//    module: prometheus
	//    heritage: deckhouse
	//    app: prometheus
	//    prometheus: main
	//    component: rules
	// spec:
	//  groups:
	// $(echo "$rule" | yq r - | sed 's/^/  /')

	customName := fmt.Sprintf("d8-custom-%s", name)

	un := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       "PrometheusRule",
		"metadata": map[string]interface{}{
			"name":      customName,
			"namespace": "d8-monitoring",
			"labels": map[string]interface{}{
				"module":     "prometheus",
				"heritage":   "deckhouse",
				"app":        "prometheus",
				"prometheus": "main",
				"component":  "rules",
			},
		},
		"spec": map[string]interface{}{
			"groups": groups,
		},
	}}

	return un
}
