package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, enableExtendedMonitoring)

func enableExtendedMonitoring(input *go_hook.HookInput) error {
	annotationsPatch := v1.ObjectMeta{Annotations: map[string]string{"extended-monitoring.flant.com/enabled": ""}}
	jsonPatch, err := json.Marshal(annotationsPatch)
	if err != nil {
		return err
	}

	err = input.ObjectPatcher.MergePatchObject(jsonPatch, "v1", "namespace", "", "d8-system", "")
	if err != nil {
		return err
	}

	err = input.ObjectPatcher.MergePatchObject(jsonPatch, "v1", "namespace", "", "kube-system", "")
	if err != nil {
		return err
	}

	return nil
}
