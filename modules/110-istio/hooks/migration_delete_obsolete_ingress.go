/*
Copyright 2024 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	istioNs         = "d8-istio"
	obsoleteIngress = "kiali-rewrite"
)

// This hook deletes d8-istio/kiali-rewrite obsolete ingress
// TODO: Remove this hook after 1.65

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(deleteIngress))

func deleteIngress(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	kubeClient := dc.MustGetK8sClient()

	if err := kubeClient.NetworkingV1().Ingresses(istioNs).Delete(context.Background(), obsoleteIngress, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
