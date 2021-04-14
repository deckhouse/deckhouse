package hooks

import (
	"fmt"
	"io/ioutil"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, discoverApiserverCA)

func discoverApiserverCA(input *go_hook.HookInput) error {
	caPath := "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	content, err := ioutil.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("cannot find kubernetes ca: %v, (not in pod?)", err)
	}

	input.Values.Set("userAuthn.internal.kubernetesCA", string(content))
	return nil
}
