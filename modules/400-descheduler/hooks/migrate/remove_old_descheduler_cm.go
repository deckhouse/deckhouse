/*
Copyright 2022 Flant JSC

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(removeOldDeschedulerSecret))

func removeOldDeschedulerSecret(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	err = kubeCl.CoreV1().ConfigMaps("d8-system").Delete(context.TODO(), "descheduler-config-migration", metav1.DeleteOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			input.Logger.Info("Descheduler legacy CM d8-system/descheduler-config-migration is not found, skipping migration")
			return nil
		}
		return err
	}

	input.Logger.Info("Descheduler legacy CM d8-system/descheduler-config-migration is deleted")
	return nil
}
