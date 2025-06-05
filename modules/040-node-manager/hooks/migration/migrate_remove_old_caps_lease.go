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
	d8CapsNs           = "d8-cloud-instance-manager"
	d8CapsLeaseNameOld = "faf94607.cluster.x-k8s.io"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(removeOldCapsLease))

func removeOldCapsLease(_ *go_hook.HookInput, dc dependency.Container) error {
	kubeClient := dc.MustGetK8sClient()

	err := kubeClient.CoordinationV1().Leases(d8CapsNs).Delete(context.Background(), d8CapsLeaseNameOld, metav1.DeleteOptions{})

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
