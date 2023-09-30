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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

/* Migration: Delete after Deckhouse release 1.53
This migration is implemented as a global hook because it must happen
before the rolling update of the validating webhook from the 002-deckhouse module.
Otherwise, the webhook will prevent any interactions with ExternalModule* resources.
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 15},
}, dependency.WithExternalDependencies(modulesCRMigrate))

func modulesCRMigrate(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	modulesMigrationCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "modules-cr-names-migration",
			Namespace: "d8-system",
		},
	}

	_, err = kubeCl.CoreV1().ConfigMaps(modulesMigrationCM.Namespace).Get(context.TODO(), modulesMigrationCM.Name, metav1.GetOptions{})
	if err == nil {
		input.LogEntry.Info("Modules migration configmap exists, skipping the migration")
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// old
	emsGVR := schema.ParseGroupResource("externalmodulesources.deckhouse.io").WithVersion("v1alpha1")
	emrGVR := schema.ParseGroupResource("externalmodulereleases.deckhouse.io").WithVersion("v1alpha1")
	// new
	msGVR := schema.ParseGroupResource("modulesources.deckhouse.io").WithVersion("v1alpha1")
	mrGVR := schema.ParseGroupResource("modulereleases.deckhouse.io").WithVersion("v1alpha1")

	skipMigrationCount := 0

	moduleSources, err := kubeCl.Dynamic().Resource(emsGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		input.LogEntry.Info("ExternalModuleSource resource is not in the cluster")
		skipMigrationCount++
	}

	moduleReleases, err := kubeCl.Dynamic().Resource(emrGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		input.LogEntry.Info("ExternalModuleRelease resource is not in the cluster")
		skipMigrationCount++
	}

	if skipMigrationCount == 2 { // both resources are absent
		input.LogEntry.Info("Skipping modules migration")
		return nil
	}

	if moduleSources != nil {
		for _, ms := range moduleSources.Items {
			sanitizeUnstructured("ModuleSource", &ms)

			_, err := kubeCl.Dynamic().Resource(msGVR).Create(context.TODO(), &ms, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	if moduleReleases != nil {
		for _, mr := range moduleReleases.Items {
			sanitizeUnstructured("ModuleRelease", &mr)

			_, err := kubeCl.Dynamic().Resource(mrGVR).Create(context.TODO(), &mr, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	err = kubeCl.Dynamic().Resource(emsGVR).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
	if err != nil {
		input.LogEntry.Info("cannot delete external module sources", err)
		return nil
	}

	err = kubeCl.Dynamic().Resource(emrGVR).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
	if err != nil {
		input.LogEntry.Info("cannot delete external module releases", err)
		return nil
	}

	_, err = kubeCl.CoreV1().ConfigMaps(modulesMigrationCM.Namespace).Create(context.TODO(), modulesMigrationCM, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Remove fields like resource version because otherwise the create requests will lead to an error
func sanitizeUnstructured(kind string, o *unstructured.Unstructured) {
	o.SetKind(kind)
	o.Object["metadata"] = map[string]interface{}{
		"name":        o.GetName(),
		"namespace":   o.GetNamespace(),
		"labels":      o.GetLabels(),
		"annotations": o.GetAnnotations(),
	}
}
