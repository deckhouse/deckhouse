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
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/log-shipper/generate_config",
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

type ClusterLogDestination struct {
	Name string
	Spec interface{}
}

func handleClusterLogs(input *go_hook.HookInput) error {
	destMap := make(map[string]v1alpha1.ClusterLogDestination)
	snap := input.Snapshots["cluster_log_destination"]

	// load all destinations into the destination map
	for _, d := range snap {
		dest := d.(v1alpha1.ClusterLogDestination)
		destMap[dest.Name] = dest
	}

	snap = input.Snapshots["cluster_log_source"]
	clusterSources := make([]vector.ClusterLoggingConfig, 0, len(snap))
	for _, s := range snap {
		tmpSpec := s.(v1alpha1.ClusterLoggingConfig)
		sourceConfig := vector.ClusterLoggingConfig{
			TypeMeta:   tmpSpec.TypeMeta,
			ObjectMeta: tmpSpec.ObjectMeta,
			Spec: vector.ClusterLoggingConfigSpec{
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
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, vector.CreateMultiLineTransforms(tmpSpec.Spec.MultiLineParser.Type)...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, vector.CreateDefaultTransforms(destMap[dest])...)
				filterTransforms, err := vector.CreateTransformsFromFilter(tmpSpec.Spec.LogFilters)
				if err != nil {
					return err
				}
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, filterTransforms...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, vector.CreateDefaultCleanUpTransforms(destMap[dest])...)
				clusterSources = append(clusterSources, newSource)
			}
		} else {
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, vector.CreateMultiLineTransforms(tmpSpec.Spec.MultiLineParser.Type)...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, vector.CreateDefaultTransforms(destMap[sourceConfig.Spec.DestinationRefs[0]])...)
			filterTransforms, err := vector.CreateTransformsFromFilter(tmpSpec.Spec.LogFilters)
			if err != nil {
				return err
			}
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, filterTransforms...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, vector.CreateDefaultCleanUpTransforms(destMap[sourceConfig.Spec.DestinationRefs[0]])...)
			clusterSources = append(clusterSources, sourceConfig)
		}
	}

	snap = input.Snapshots["namespaced_log_source"]
	namespacedSources := make([]vector.PodLoggingConfig, 0, len(snap))
	for _, s := range snap {
		tmpPogSpec := s.(v1alpha1.PodLoggingConfig)
		sourceConfig := vector.PodLoggingConfig{
			TypeMeta:   tmpPogSpec.TypeMeta,
			ObjectMeta: tmpPogSpec.ObjectMeta,
			Spec: vector.PodLoggingConfigSpec{
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
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, vector.CreateMultiLineTransforms(tmpPogSpec.Spec.MultiLineParser.Type)...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, vector.CreateDefaultTransforms(destMap[dest])...)
				filterTransforms, err := vector.CreateTransformsFromFilter(tmpPogSpec.Spec.LogFilters)
				if err != nil {
					return err
				}
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, filterTransforms...)
				newSource.Spec.Transforms = append(newSource.Spec.Transforms, vector.CreateDefaultCleanUpTransforms(destMap[dest])...)
				namespacedSources = append(namespacedSources, newSource)
			}
		} else {
			filterTransforms, err := vector.CreateTransformsFromFilter(tmpPogSpec.Spec.LogFilters)
			if err != nil {
				return err
			}
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, vector.CreateMultiLineTransforms(tmpPogSpec.Spec.MultiLineParser.Type)...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, vector.CreateDefaultTransforms(destMap[sourceConfig.Spec.ClusterDestinationRefs[0]])...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, filterTransforms...)
			sourceConfig.Spec.Transforms = append(sourceConfig.Spec.Transforms, vector.CreateDefaultCleanUpTransforms(destMap[sourceConfig.Spec.ClusterDestinationRefs[0]])...)
			namespacedSources = append(namespacedSources, sourceConfig)
		}
	}

	generator := vector.NewLogConfigGenerator()
	var generatedPipelines int

	for _, source := range clusterSources {
		source, transforms, destinations, err := pipelinePartsFromClusterSource(generator, destMap, &source)
		if err != nil {
			input.LogEntry.Warn(err)
			continue
		}

		generator.AppendLogPipeline(source, transforms, destinations)
		generatedPipelines++
	}

	for _, source := range namespacedSources {
		source, transforms, destinations, err := pipelinePartsFromNamespacedSource(generator, destMap, &source)
		if err != nil {
			input.LogEntry.Warn(err)
			continue
		}

		generator.AppendLogPipeline(source, transforms, destinations)
		generatedPipelines++
	}

	if generatedPipelines == 0 {
		input.Values.Set("logShipper.internal.activated", false)
		input.PatchCollector.Delete("v1", "Secret", "d8-log-shipper", "d8-log-shipper-config", object_patch.InBackground())
		return nil
	}

	config, err := generator.GenerateConfig()
	if err != nil {
		return err
	}

	// set activated value
	input.Values.Set("logShipper.internal.activated", true)

	// create secret with configuration
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
		Data: map[string][]byte{"vector.json": config},
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

func pipelinePartsFromClusterSource(generator *vector.LogConfigGenerator, destMap map[string]v1alpha1.ClusterLogDestination, sourceConfig *vector.ClusterLoggingConfig) (source impl.LogSource, transforms []impl.LogTransform, destinations []impl.LogDestination, err error) {
	// for each source looking for all destinations
	for _, dstRef := range sourceConfig.Spec.DestinationRefs {
		cdest, ok := destMap[dstRef]
		if !ok {
			err = fmt.Errorf("destinationRef: %s for ClusterLoggingConfig: %s not found. Skipping", dstRef, sourceConfig.Name)
			return
		}
		dest := newLogDest(cdest.Spec.Type, cdest.Name, cdest.Spec)
		destinations = append(destinations, dest)
	}

	if len(sourceConfig.Spec.Transforms) > 0 {
		transforms, err = generator.BuildTransformsFromMapSlice(sourceConfig.Name, sourceConfig.Spec.Transforms)
		if err != nil {
			err = errors.Wrap(err, "transforms build from snippet failed. Skipping")
			return
		}
	}

	source = newLogSource(sourceConfig.Spec.Type, sourceConfig.Name, sourceConfig.Spec)

	return
}

func pipelinePartsFromNamespacedSource(generator *vector.LogConfigGenerator, destMap map[string]v1alpha1.ClusterLogDestination, sourceConfig *vector.PodLoggingConfig) (source impl.LogSource, transforms []impl.LogTransform, destinations []impl.LogDestination, err error) {
	// for each source looking for all cluster destinations
	for _, dstRef := range sourceConfig.Spec.ClusterDestinationRefs {
		cdest, ok := destMap[dstRef]
		if !ok {
			err = fmt.Errorf("clusterDestinationRef: %s for PodLoggingConfig: %s not found. Skipping", dstRef, sourceConfig.Name)
			return
		}
		dest := newLogDest(cdest.Spec.Type, cdest.Name, cdest.Spec)
		destinations = append(destinations, dest)
	}

	namespacedName := fmt.Sprintf("%s_%s", sourceConfig.Namespace, sourceConfig.Name)
	// prefer snippet over structured transforms
	if len(sourceConfig.Spec.Transforms) > 0 {
		transforms, err = generator.BuildTransformsFromMapSlice(namespacedName, sourceConfig.Spec.Transforms)
		if err != nil {
			err = errors.Wrap(err, "transforms build from snippet failed. Skipping")
			return
		}
	}

	// set namespace selector to config namespace. It's only 1 namespace available for Namespaced config
	kubeSpec := v1alpha1.KubernetesPodsSpec{
		NamespaceSelector: v1alpha1.NamespaceSelector{MatchNames: []string{sourceConfig.Namespace}},
		LabelSelector:     sourceConfig.Spec.LabelSelector,
	}
	source = vector.NewKubernetesLogSource(sourceConfig.Name, kubeSpec, true)

	return
}

func newLogSource(typ, name string, spec vector.ClusterLoggingConfigSpec) impl.LogSource {
	switch typ {
	case vector.SourceFile:
		return vector.NewFileLogSource(name, spec.File)

	case vector.SourceKubernetesPods:
		return vector.NewKubernetesLogSource(name, spec.KubernetesPods, false)

	default:
		return nil
	}
}

func newLogDest(typ, name string, spec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	switch typ {
	case vector.DestLoki:
		return vector.NewLokiDestination(name, spec)
	case vector.DestElasticsearch:
		return vector.NewElasticsearchDestination(name, spec)
	case vector.DestLogstash:
		return vector.NewLogstashDestination(name, spec)

	default:
		return nil
	}
}
