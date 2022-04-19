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
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/config"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/model"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transform"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/log-shipper/generate_config",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_log_source",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLoggingConfig",
			FilterFunc: filterClusterLoggingConfig,
		},
		{
			Name:       "namespaced_log_source",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "PodLoggingConfig",
			FilterFunc: filterPodLoggingConfig,
		},
		{
			Name:       "cluster_log_destination",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLogDestination",
			FilterFunc: filterClusterLogDestination,
		},
	},
}, handleClusterLogs)

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

func handleClusterLogs(input *go_hook.HookInput) error {
	destNameToObject := make(map[string]v1alpha1.ClusterLogDestination)
	snap := input.Snapshots["cluster_log_destination"]

	// load all destinations into the destination map
	for _, d := range snap {
		dest := d.(v1alpha1.ClusterLogDestination)
		destNameToObject[dest.Name] = dest
	}

	snap = input.Snapshots["cluster_log_source"]
	clusterSources := make([]model.ClusterLoggingConfig, 0, len(snap))

	// TODO(nabokihms): refactor these blocks
	for _, s := range snap {
		tmpSpec := s.(v1alpha1.ClusterLoggingConfig)
		sourceConfig := model.ClusterLoggingConfig{
			TypeMeta:   tmpSpec.TypeMeta,
			ObjectMeta: tmpSpec.ObjectMeta,
			Spec: model.ClusterLoggingConfigSpec{
				Type:            tmpSpec.Spec.Type,
				KubernetesPods:  tmpSpec.Spec.KubernetesPods,
				File:            tmpSpec.Spec.File,
				DestinationRefs: tmpSpec.Spec.DestinationRefs,
			},
			Status: tmpSpec.Status,
		}
		if len(sourceConfig.Spec.DestinationRefs) > 1 {
			for _, dest := range sourceConfig.Spec.DestinationRefs {
				newSource := sourceConfig
				newSource.Name = sourceConfig.Name + "_" + dest
				newSource.Spec.DestinationRefs = make([]string, 1)
				newSource.Spec.DestinationRefs[0] = dest
				newSource.Spec.Transforms = make([]impl.LogTransform, 0)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, transform.CreateMultiLineTransforms(tmpSpec.Spec.MultiLineParser.Type)...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, transform.CreateDefaultTransforms(destNameToObject[dest])...)
				filterTransforms, err := transform.CreateLogFilterTransforms(tmpSpec.Spec.LogFilters)
				if err != nil {
					return err
				}
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, filterTransforms...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, transform.CreateDefaultCleanUpTransforms(destNameToObject[dest])...)
				clusterSources = append(clusterSources, newSource)
			}
		} else {
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, transform.CreateMultiLineTransforms(tmpSpec.Spec.MultiLineParser.Type)...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, transform.CreateDefaultTransforms(destNameToObject[sourceConfig.Spec.DestinationRefs[0]])...)
			filterTransforms, err := transform.CreateLogFilterTransforms(tmpSpec.Spec.LogFilters)
			if err != nil {
				return err
			}
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, filterTransforms...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, transform.CreateDefaultCleanUpTransforms(destNameToObject[sourceConfig.Spec.DestinationRefs[0]])...)
			clusterSources = append(clusterSources, sourceConfig)
		}
	}

	snap = input.Snapshots["namespaced_log_source"]
	namespacedSources := make([]model.PodLoggingConfig, 0, len(snap))

	// TODO(nabokihms): refactor these blocks
	for _, s := range snap {
		tmpPogSpec := s.(v1alpha1.PodLoggingConfig)
		sourceConfig := model.PodLoggingConfig{
			TypeMeta:   tmpPogSpec.TypeMeta,
			ObjectMeta: tmpPogSpec.ObjectMeta,
			Spec: model.PodLoggingConfigSpec{
				LabelSelector:          tmpPogSpec.Spec.LabelSelector,
				ClusterDestinationRefs: tmpPogSpec.Spec.ClusterDestinationRefs,
			},
			Status: tmpPogSpec.Status,
		}
		if len(sourceConfig.Spec.ClusterDestinationRefs) > 1 {
			for _, dest := range sourceConfig.Spec.ClusterDestinationRefs {
				newSource := sourceConfig
				newSource.Name = sourceConfig.Name + "_" + dest
				newSource.Spec.ClusterDestinationRefs = make([]string, 1)
				newSource.Spec.ClusterDestinationRefs[0] = dest
				newSource.Spec.Transforms = make([]impl.LogTransform, 0)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, transform.CreateMultiLineTransforms(tmpPogSpec.Spec.MultiLineParser.Type)...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, transform.CreateDefaultTransforms(destNameToObject[dest])...)
				filterTransforms, err := transform.CreateLogFilterTransforms(tmpPogSpec.Spec.LogFilters)
				if err != nil {
					return err
				}
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, filterTransforms...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, transform.CreateDefaultCleanUpTransforms(destNameToObject[dest])...)
				namespacedSources = append(namespacedSources, newSource)
			}
		} else {
			filterTransforms, err := transform.CreateLogFilterTransforms(tmpPogSpec.Spec.LogFilters)
			if err != nil {
				return err
			}
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, transform.CreateMultiLineTransforms(tmpPogSpec.Spec.MultiLineParser.Type)...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, transform.CreateDefaultTransforms(destNameToObject[sourceConfig.Spec.ClusterDestinationRefs[0]])...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, filterTransforms...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, transform.CreateDefaultCleanUpTransforms(destNameToObject[sourceConfig.Spec.ClusterDestinationRefs[0]])...)
			namespacedSources = append(namespacedSources, sourceConfig)
		}
	}

	generator := config.NewLogConfigGenerator()
	var logShipperActivated bool

	for _, source := range clusterSources {
		pipeline, err := config.NewPipelineCluster(generator, destNameToObject, &source)
		if err != nil {
			input.LogEntry.Warn(err)
			continue
		}
		generator.AppendLogPipeline(pipeline)
		logShipperActivated = true
	}

	for _, source := range namespacedSources {
		pipeline, err := config.NewPipelineNamespaced(generator, destNameToObject, &source)
		if err != nil {
			input.LogEntry.Warn(err)
			continue
		}
		generator.AppendLogPipeline(pipeline)
		logShipperActivated = true
	}

	input.Values.Set("logShipper.internal.activated", logShipperActivated)

	if logShipperActivated {
		configFile, err := generator.GenerateConfig()
		if err != nil {
			return err
		}

		createOrUpdateConfigAndEvent(input, configFile)
		return nil
	}

	input.PatchCollector.Delete("v1", "Secret", "d8-log-shipper", "d8-log-shipper-config", object_patch.InBackground())
	return nil
}

func createOrUpdateConfigAndEvent(input *go_hook.HookInput, configFile []byte) {
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
		Data: map[string][]byte{"vector.json": configFile},
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
}
