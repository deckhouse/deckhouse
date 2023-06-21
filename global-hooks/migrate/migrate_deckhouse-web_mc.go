/*
Copyright 2023 Flant JSC

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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var (
	mcGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "moduleconfigs",
	}
)
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(mcMigration))

func mcMigration(_ *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	mc, err := kubeCl.Dynamic().Resource(mcGVR).Get(context.TODO(), "deckhouse-web", v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	unstructured.RemoveNestedField(mc.Object, "auth", "password")
	err = unstructured.SetNestedField(mc.Object, 1, "spec", "version")
	if err != nil {
		return err
	}

	err = unstructured.SetNestedField(mc.Object, "documentation", "metadata", "name")
	if err != nil {
		return err
	}

	_, err = kubeCl.Dynamic().Resource(mcGVR).Create(context.TODO(), mc, v1.CreateOptions{})
	if err != nil {
		return err
	}

	return kubeCl.Dynamic().Resource(mcGVR).Delete(context.TODO(), "deckhouse-web", v1.DeleteOptions{})
}
