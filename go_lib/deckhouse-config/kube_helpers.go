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

package deckhouse_config

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const GeneratedConfigMapName = "deckhouse-generated-config-do-not-edit"
const DeckhouseConfigMapName = "deckhouse"
const DeckhouseNS = "d8-system"

// GetGeneratedConfigMap returns generated ConfigMap with config values.
func GetGeneratedConfigMap(klient k8s.Client) (*v1.ConfigMap, error) {
	return GetConfigMap(klient, DeckhouseNS, GeneratedConfigMapName)
}

// GetDeckhouseConfigMap returns default ConfigMap with config values (ConfigMap/deckhouse).
func GetDeckhouseConfigMap(klient k8s.Client) (*v1.ConfigMap, error) {
	return GetConfigMap(klient, DeckhouseNS, DeckhouseConfigMapName)
}

func GeneratedConfigMap(data map[string]string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GeneratedConfigMapName,
			Namespace: DeckhouseNS,
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Data: data,
	}
}

func GetConfigMap(klient k8s.Client, ns string, name string) (*v1.ConfigMap, error) {
	cmUntyped, err := klient.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}).Namespace(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var cm v1.ConfigMap
	err = sdk.FromUnstructured(cmUntyped, &cm)
	if err != nil {
		return nil, err
	}
	return &cm, nil
}

// GetAllConfigs returns all ModuleConfig objects.
func GetAllConfigs(kubeClient k8s.Client) ([]*v1alpha1.ModuleConfig, error) {
	gvr := v1alpha1.ModuleConfigGVR
	unstructuredObjs, err := kubeClient.Dynamic().Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	objs := make([]*v1alpha1.ModuleConfig, 0, len(unstructuredObjs.Items))
	for _, unst := range unstructuredObjs.Items {
		var obj v1alpha1.ModuleConfig

		err := sdk.FromUnstructured(&unst, &obj)
		if err != nil {
			return nil, err
		}

		objs = append(objs, &obj)
	}

	return objs, nil
}

// SetModuleConfigEnabledFlag updates spec.enabled flag or creates a new ModuleConfig with spec.enabled flag.
func SetModuleConfigEnabledFlag(kubeClient k8s.Client, name string, enabled bool) error {
	// This should not happen, but check it anyway.
	if kubeClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	gvr := v1alpha1.ModuleConfigGVR
	unstructuredObj, err := kubeClient.Dynamic().Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && !k8errors.IsNotFound(err) {
		return fmt.Errorf("get ModuleConfig/%s: %w", name, err)
	}

	if unstructuredObj != nil {
		err := unstructured.SetNestedField(unstructuredObj.Object, enabled, "spec", "enabled")
		if err != nil {
			return fmt.Errorf("change spec.enabled to %v in ModuleConfig/%s: %w", enabled, name, err)
		}
		_, err = kubeClient.Dynamic().Resource(gvr).Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update ModuleConfig/%s: %w", name, err)
		}
		return nil
	}

	// Create new ModuleConfig if absent.
	newCfg := &v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleConfigGVK.Kind,
			APIVersion: v1alpha1.ModuleConfigGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled: pointer.Bool(enabled),
		},
	}

	obj, err := sdk.ToUnstructured(newCfg)
	if err != nil {
		return fmt.Errorf("converting ModuleConfig/%s to unstructured: %w", name, err)
	}

	_, err = kubeClient.Dynamic().Resource(gvr).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create ModuleConfig/%s: %w", name, err)
	}
	return nil
}
