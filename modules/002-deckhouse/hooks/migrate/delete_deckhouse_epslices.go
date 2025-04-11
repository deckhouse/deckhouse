/*
Copyright 2025 Flant JSC

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

package migrate

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(removeDeckhouseEpslices))

func removeDeckhouseEpslices(_ *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get kubernetes client: %v", err)
	}

	return kubeClient.DiscoveryV1().EndpointSlices("d8-system").Delete(context.TODO(), "deckhouse", metav1.DeleteOptions{})
}
