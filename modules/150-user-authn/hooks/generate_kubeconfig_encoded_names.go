package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, kubeconfigNamesHandler)

func kubeconfigNamesHandler(input *go_hook.HookInput) error {
	const (
		kubeconfigsPath  = "userAuthn.kubeconfigGenerator"
		encodedNamesPath = "userAuthn.internal.kubeconfigEncodedNames"
	)

	if !input.ConfigValues.Exists(kubeconfigsPath) {
		if input.ConfigValues.Exists(encodedNamesPath) {
			input.Values.Remove(encodedNamesPath)
		}
		return nil
	}

	kubeconfigsLength, err := input.ConfigValues.ArrayCount(kubeconfigsPath)
	if err != nil {
		return fmt.Errorf("get %s length: %v", kubeconfigsPath, err)
	}

	encodedNames := make([]string, 0, kubeconfigsLength)

	for i := 0; i < kubeconfigsLength; i++ {
		name := encoding.ToFnvLikeDex(fmt.Sprintf("kubeconfig-generator-%d", i))
		encodedNames = append(encodedNames, name)
	}

	input.Values.Set(encodedNamesPath, encodedNames)
	return nil
}
