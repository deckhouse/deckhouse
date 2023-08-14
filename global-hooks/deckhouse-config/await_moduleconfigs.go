// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const bootstrapConfigMapName = "deckhouse-bootstrap-lock"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 2},
}, dependency.WithExternalDependencies(handleBootstrapConfigMapExistence))

func handleBootstrapConfigMapExistence(_ *go_hook.HookInput, dc dependency.Container) error {
	ctx := context.Background()
	kube, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("dc.GetK8sClient: %w", err)
	}

	_, err = kube.CoreV1().ConfigMaps("d8-system").Get(ctx, bootstrapConfigMapName, v1.GetOptions{})
	switch {
	case err != nil && k8serror.IsNotFound(err):
		// Bootstrap lock lifted, can continue with hooks now.
		return nil
	case err != nil:
		return fmt.Errorf("cannot get %q ConfigMap data: %w", bootstrapConfigMapName, err)
	default:
		return errors.New("will wait for bootstrap module configs creation")
	}
}
