/*
Copyright 2021 Flant JSC

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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/composer"
)

func filterPodLoggingConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var src v1alpha1.PodLoggingConfig

	err := sdk.FromUnstructured(obj, &src)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func filterClusterLoggingConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var src v1alpha1.ClusterLoggingConfig

	err := sdk.FromUnstructured(obj, &src)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func filterClusterLogDestination(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dst v1alpha1.ClusterLogDestination

	err := sdk.FromUnstructured(obj, &dst)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

func filterNamespaceName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var namespace corev1.Namespace

	err := sdk.FromUnstructured(obj, &namespace)
	if err != nil {
		return nil, err
	}
	return namespace.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/log-shipper/generate_config",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaced_log_source",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "PodLoggingConfig",
			FilterFunc: filterPodLoggingConfig,
		},
		{
			Name:       "cluster_log_source",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLoggingConfig",
			FilterFunc: filterClusterLoggingConfig,
		},
		{
			Name:       "cluster_log_destination",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLogDestination",
			FilterFunc: filterClusterLogDestination,
		},
		{
			Name:       "namespace",
			ApiVersion: "v1",
			Kind:       "Namespace",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-log-shipper"},
			},
			FilterFunc: filterNamespaceName,
		},
	},
}, generateConfig)

func generateConfig(input *go_hook.HookInput) error {
	if len(input.Snapshots["namespace"]) < 1 {
		// there is no namespace to manipulate the config map, the hook will create it later on afterHelm
		input.Values.Set("logShipper.internal.activated", false)
		return nil
	}

	configContent, err := composer.FromInput(input).Do()
	if err != nil {
		return err
	}

	activated := len(configContent) != 0
	input.Values.Set("logShipper.internal.activated", activated)

	if !activated {
		input.PatchCollector.Delete(
			"v1", "Secret", "d8-log-shipper", "d8-log-shipper-config",
			object_patch.InBackground())
		return nil
	}

	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-log-shipper-config",
			Namespace: "d8-log-shipper",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "log-shipper",
			},
		},
		Data: map[string][]byte{"vector.json": configContent},
	}
	input.PatchCollector.Create(secret, object_patch.UpdateIfExists())

	event := &eventsv1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    secret.Namespace,
			GenerateName: secret.Name + "-",
		},
		Regarding: corev1.ObjectReference{
			Kind:       secret.Kind,
			Name:       secret.Name,
			Namespace:  secret.Namespace,
			APIVersion: secret.APIVersion,
		},
		Reason:              "LogShipperConfigCreateUpdate",
		Note:                "Config file has been created or updated.",
		Action:              "Create/Update",
		Type:                corev1.EventTypeNormal,
		EventTime:           metav1.MicroTime{Time: time.Now()},
		ReportingInstance:   "deckhouse",
		ReportingController: "deckhouse",
	}
	input.PatchCollector.Create(event)

	return nil
}
