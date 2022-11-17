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

	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	d8cfg_v1alpha1 "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/v1alpha1"
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
func GetAllConfigs(kubeClient k8s.Client) ([]*d8cfg_v1alpha1.ModuleConfig, error) {
	gvr := d8cfg_v1alpha1.GroupVersionResource()
	unstructuredObjs, err := kubeClient.Dynamic().Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	objs := make([]*d8cfg_v1alpha1.ModuleConfig, 0, len(unstructuredObjs.Items))
	for _, unstructured := range unstructuredObjs.Items {
		var obj d8cfg_v1alpha1.ModuleConfig

		err := sdk.FromUnstructured(&unstructured, &obj)
		if err != nil {
			return nil, err
		}

		objs = append(objs, &obj)
	}

	return objs, nil
}
