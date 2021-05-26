package hooks

import (
	"io/ioutil"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, discoverKubernetesCAHandler)

const rootCAFile = "/run/secrets/kubernetes.io/serviceaccount/ca.crt"

func discoverKubernetesCAHandler(input *go_hook.HookInput) error {
	caBytes, err := ioutil.ReadFile(rootCAFile)
	if err != nil {
		return err
	}

	input.Values.Set("nodeManager.internal.kubernetesCA", string(caBytes))
	return nil
}
