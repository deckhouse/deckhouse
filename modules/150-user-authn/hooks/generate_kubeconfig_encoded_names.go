/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, kubeconfigNamesHandler)

func kubeconfigNamesHandler(_ context.Context, input *go_hook.HookInput) error {
	const (
		kubeconfigsPath  = "userAuthn.kubeconfigGenerator"
		encodedNamesPath = "userAuthn.internal.kubeconfigEncodedNames"
	)

	if !input.ConfigValues.Exists(kubeconfigsPath) {
		input.Values.Remove(encodedNamesPath)
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
