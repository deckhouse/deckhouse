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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// TODO: remove this hook after Deckhouse 1.50

// this hook reads ModuleConfig/deckhouse-web if exists and creates new ModuleConfig/documentation for renamed module
// it creates the prometheus alert to warn users about CI migrations to be done

var (
	mcGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "moduleconfigs",
	}
)
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(documentationMCMigration))

func documentationMCMigration(input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire("d8_mc")

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

	unstructured.RemoveNestedField(mc.Object, "spec", "settings", "auth", "password")
	unstructured.RemoveNestedField(mc.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(mc.Object, "metadata", "generation")
	unstructured.RemoveNestedField(mc.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(mc.Object, "metadata", "uid")
	unstructured.RemoveNestedField(mc.Object, "status")

	err = unstructured.SetNestedField(mc.Object, int64(1), "spec", "version")
	if err != nil {
		return err
	}

	err = unstructured.SetNestedField(mc.Object, "documentation", "metadata", "name")
	if err != nil {
		return err
	}

	_, err = kubeCl.Dynamic().Resource(mcGVR).Create(context.TODO(), mc, v1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return kubeCl.Dynamic().Resource(mcGVR).Delete(context.TODO(), "deckhouse-web", v1.DeleteOptions{})
}
