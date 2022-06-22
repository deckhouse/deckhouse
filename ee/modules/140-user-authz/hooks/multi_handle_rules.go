/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"regexp"

	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/140-user-authz/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: filterNS,
		},
		{
			Name:       "authrules",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: filterRule,
		},
	},
}, handleBindings)

func filterNS(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func filterRule(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	fmt.Println("FRULE", obj.GetName())

	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return nil, fmt.Errorf(`".spec is not a map[string]interface{} or contains non-string values in the map: %s`, spew.Sdump(obj.Object))
	}
	if err != nil {
		return nil, err
	}

	var isMultitenancy bool

	if _, ok := spec["allowAccessToSystemNamespaces"]; ok {
		fmt.Println("HAS ALLOW")
		isMultitenancy = true
	}

	if _, ok := spec["limitNamespaces"]; ok {
		fmt.Println("HAS LIMIT")
		isMultitenancy = true
	}

	if !isMultitenancy {
		return nil, nil
	}

	var rule internal.ClusterAuthorizationRule

	err = sdk.FromUnstructured(obj, &rule)
	if err != nil {
		return nil, err
	}

	return &rule, nil
}

var (
	systemNSRegexp = []string{"kube-.*", "d8-.*", "loghouse", "default"}
)

func handleBindings(input *go_hook.HookInput) error {
	multitenancyRules := make([]openAPIRule, 0)

	val, ok := input.Values.GetOk("userAuthz.enableMultiTenancy")
	if !ok || !val.Bool() {
		input.Values.Set("userAuthz.internal.multitenancyCRDs", multitenancyRules)
		return nil
	}

	snap := input.Snapshots["namespaces"]
	allNamespaces := make([]string, 0, len(snap))
	for _, ns := range snap {
		allNamespaces = append(allNamespaces, ns.(string))
	}

	snap = input.Snapshots["authrules"]

	for _, sn := range snap {
		if sn == nil {
			continue
		}
		rule := sn.(*internal.ClusterAuthorizationRule)
		nsRegexps := make([]string, 0)
		if len(rule.Spec.LimitNamespaces) > 0 {
			nsRegexps = append(nsRegexps, rule.Spec.LimitNamespaces...)
		}

		if rule.Spec.AllowAccessToSystemNamespaces {
			nsRegexps = append(nsRegexps, systemNSRegexp...)
		}

		if len(nsRegexps) == 0 {
			continue
		}

		calculatedNamespaces := make([]string, 0)

		for _, regns := range nsRegexps {
			reg, err := regexp.Compile(regns)
			if err != nil {
				input.LogEntry.Warnf("compile NS regexp failed: %s", err)
				continue
			}

			for _, ns := range allNamespaces {
				if reg.MatchString(ns) {
					calculatedNamespaces = append(calculatedNamespaces, ns)
				}
			}
		}

		oRule := openAPIRule{
			Name: rule.Name,
			Spec: rule.Spec,
		}
		oRule.Spec.LimitNamespaces = calculatedNamespaces

		multitenancyRules = append(multitenancyRules, oRule)
	}

	input.Values.Set("userAuthz.internal.multitenancyCRDs", multitenancyRules)

	return nil
}

type openAPIRule struct {
	Name string                                `json:"name"`
	Spec internal.ClusterAuthorizationRuleSpec `json:"spec"`
}
