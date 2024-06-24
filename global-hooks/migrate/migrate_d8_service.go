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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// TODO: Remove after release 1.63
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(migrateService))

func migrateService(_ *go_hook.HookInput, dc dependency.Container) error {
	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	svc, err := client.CoreV1().Services("d8-system").Get(context.Background(), "deckhouse", v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !svc.Spec.PublishNotReadyAddresses {
		svc.Spec.PublishNotReadyAddresses = true
		svc.Spec.Selector = map[string]string{"app": "deckhouse"}
		_, err = client.CoreV1().Services("d8-system").Update(context.Background(), svc, v1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
