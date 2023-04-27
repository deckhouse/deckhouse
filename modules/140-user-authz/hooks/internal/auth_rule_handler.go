package internal

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type authorizationRule struct {
	Name      string                 `json:"name"`
	Spec      map[string]interface{} `json:"spec"`
	Namespace string                 `json:"namespace,omitempty"`
}

func ApplyAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return nil, fmt.Errorf(`".spec is not a map[string]interface{} or contains non-string values in the map: %s`, spew.Sdump(obj.Object))
	}
	if err != nil {
		return nil, err
	}

	car := &authorizationRule{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Spec:      spec,
	}

	return car, nil
}

func AuthorizationRulesHandler(valuesPath, snapshotKey string) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		input.Values.Set(valuesPath, snapshotsToAuthorizationRulesSlice(input.Snapshots[snapshotKey]))
		return nil
	}
}

func snapshotsToAuthorizationRulesSlice(snapshots []go_hook.FilterResult) []authorizationRule {
	ars := make([]authorizationRule, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		ar := snapshot.(*authorizationRule)
		ars = append(ars, *ar)
	}
	return ars
}
