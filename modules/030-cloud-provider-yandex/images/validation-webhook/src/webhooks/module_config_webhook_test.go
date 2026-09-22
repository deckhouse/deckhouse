// Copyright 2026 Flant JSC
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

package webhooks

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

// yandexModuleConfigObject builds a ModuleConfig whose nodes.parameters.externalIPAddresses
// assigns the given addresses to the given NodeGroup.
func yandexModuleConfigObject(name string, nodeGroupName string, addresses []any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"})
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{
		"enabled": true,
		"version": int64(2),
		"settings": map[string]any{
			"provider": map[string]any{
				"parameters": map[string]any{"cloudID": "cloud-1", "folderID": "folder-1"},
			},
			"nodes": map[string]any{
				"parameters": map[string]any{
					"layout":              "Standard",
					"nodeNetworkCIDR":     "10.0.0.0/16",
					"sshPublicKey":        "ssh-rsa KEY",
					"externalIPAddresses": map[string]any{nodeGroupName: addresses},
				},
			},
		},
	}

	return obj
}

// yandexNodeGroupWithNodes builds a CloudPermanent NodeGroup with the given node count.
func yandexNodeGroupWithNodes(name string, maxPerZone int64) *unstructured.Unstructured {
	obj := yandexNodeGroupObject(name, cpapi.NodeTypeCloudPermanent)
	obj.Object["spec"] = map[string]any{
		"nodeType": string(cpapi.NodeTypeCloudPermanent),
		"cloudInstances": map[string]any{
			"maxPerZone": maxPerZone,
			"classReference": map[string]any{
				"kind": "YandexInstanceClass",
				"name": "worker-yandex",
			},
		},
	}

	return obj
}

// The reviewed object is the ModuleConfig, so the NodeGroups it is validated against can only
// come from the cluster: the state builder has to load all of them.
func TestModuleConfigValidatorLoadsNodeGroupsForExternalIPAddresses(t *testing.T) {
	t.Parallel()

	objects := append(validYandexClusterObjects(), yandexNodeGroupWithNodes("worker", 2))
	factory := newWebhookAdmissionStateBuilderFactory(t, objects...)
	validator := NewModuleConfigValidator(factory, &unstructured.Unstructured{})

	moduleConfig := yandexModuleConfigObject(ycmeta.ModuleName, "worker", []any{"1.2.3.4"})

	_, err := validator.ValidateUpdate(context.Background(), nil, moduleConfig)
	if err == nil || !strings.Contains(err.Error(), `number of nodes in NodeGroup "worker" (2)`) {
		t.Fatalf("ValidateUpdate() error = %v, want external IP addresses denial", err)
	}
}

func TestModuleConfigValidatorAllowsEnoughExternalIPAddresses(t *testing.T) {
	t.Parallel()

	objects := append(validYandexClusterObjects(), yandexNodeGroupWithNodes("worker", 2))
	factory := newWebhookAdmissionStateBuilderFactory(t, objects...)
	validator := NewModuleConfigValidator(factory, &unstructured.Unstructured{})

	moduleConfig := yandexModuleConfigObject(ycmeta.ModuleName, "worker", []any{"1.2.3.4", "5.6.7.8"})

	if _, err := validator.ValidateUpdate(context.Background(), nil, moduleConfig); err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestModuleConfigValidatorSkipsForeignModuleConfig(t *testing.T) {
	t.Parallel()

	objects := append(validYandexClusterObjects(), yandexNodeGroupWithNodes("worker", 2))
	factory := newWebhookAdmissionStateBuilderFactory(t, objects...)
	validator := NewModuleConfigValidator(factory, &unstructured.Unstructured{})

	moduleConfig := yandexModuleConfigObject("cloud-provider-dvp", "worker", []any{"1.2.3.4"})

	if _, err := validator.ValidateUpdate(context.Background(), nil, moduleConfig); err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want skip for another module", err)
	}
}

// The reviewed object is a NodeGroup, so the settings it is validated against can only come
// from the cluster: the state builder has to load the ModuleConfig.
func TestNodeGroupValidatorLoadsModuleConfigForExternalIPAddresses(t *testing.T) {
	t.Parallel()

	objects := append(
		validYandexClusterObjects(),
		runtime.Object(yandexModuleConfigObject(ycmeta.ModuleName, "worker", []any{"1.2.3.4"})),
	)
	factory := newWebhookAdmissionStateBuilderFactory(t, objects...)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateUpdate(context.Background(), nil, yandexNodeGroupWithNodes("worker", 2))
	if err == nil || !strings.Contains(err.Error(), `number of nodes in NodeGroup "worker" (2)`) {
		t.Fatalf("ValidateUpdate() error = %v, want external IP addresses denial", err)
	}
}
