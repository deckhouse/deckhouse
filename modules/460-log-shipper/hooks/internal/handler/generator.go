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

package handler

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/config"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/model"
)

type Generator struct {
	logDestinations map[string]v1alpha1.ClusterLogDestination
}

func FromInput(input *go_hook.HookInput) *Generator {
	destNameToObject := make(map[string]v1alpha1.ClusterLogDestination)
	snap := input.Snapshots["cluster_log_destination"]

	// load all destinations into the destination map
	for _, d := range snap {
		dest := d.(v1alpha1.ClusterLogDestination)
		destNameToObject[dest.Name] = dest
	}

	return &Generator{logDestinations: destNameToObject}
}

func (g *Generator) Do(input *go_hook.HookInput) ([]byte, error) {
	clusterSources, err := g.clusterSources(input)
	if err != nil {
		input.LogEntry.Warn(err)
	}

	namespacedSources, err := g.namespacedSources(input)
	if err != nil {
		input.LogEntry.Warn(err)
	}

	logConfigGenerator := config.NewLogConfigGenerator()

	for _, source := range clusterSources {
		pipeline, err := config.NewPipelineCluster(logConfigGenerator, g.logDestinations, &source)
		if err != nil {
			input.LogEntry.Warn(err)
			continue
		}
		logConfigGenerator.AppendLogPipeline(pipeline)
	}

	for _, source := range namespacedSources {
		pipeline, err := config.NewPipelineNamespaced(logConfigGenerator, g.logDestinations, &source)
		if err != nil {
			input.LogEntry.Warn(err)
			continue
		}
		logConfigGenerator.AppendLogPipeline(pipeline)
	}

	return logConfigGenerator.GenerateConfig()
}

func (g *Generator) convertPodLoggingConfig(podLoggingConfig v1alpha1.PodLoggingConfig) ([]model.PodLoggingConfig, error) {
	sources := make([]model.PodLoggingConfig, 0, len(podLoggingConfig.Spec.ClusterDestinationRefs))

	for _, dest := range podLoggingConfig.Spec.ClusterDestinationRefs {
		sourceConfig := model.PodLoggingConfig{
			TypeMeta: podLoggingConfig.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:      podLoggingConfig.ObjectMeta.Name + "_" + dest,
				Namespace: podLoggingConfig.ObjectMeta.Namespace,
			},
			Spec: model.PodLoggingConfigSpec{
				LabelSelector:          podLoggingConfig.Spec.LabelSelector,
				ClusterDestinationRefs: []string{dest},
			},
			Status: podLoggingConfig.Status,
		}

		applier := transformApplier{
			destination:   g.logDestinations[dest],
			labelFilter:   podLoggingConfig.Spec.LabelFilters,
			logFilter:     podLoggingConfig.Spec.LogFilters,
			multilineType: podLoggingConfig.Spec.MultiLineParser.Type,
		}

		transforms, err := applier.Do(sourceConfig.Spec.Transforms)
		if err != nil {
			return nil, err
		}

		sourceConfig.Spec.Transforms = transforms
		sources = append(sources, sourceConfig)
	}

	return sources, nil
}

func (g *Generator) convertClusterLoggingConfig(clusterLoggingConfig v1alpha1.ClusterLoggingConfig) ([]model.ClusterLoggingConfig, error) {
	sources := make([]model.ClusterLoggingConfig, 0, len(clusterLoggingConfig.Spec.DestinationRefs))

	for _, dest := range clusterLoggingConfig.Spec.DestinationRefs {
		sourceConfig := model.ClusterLoggingConfig{
			TypeMeta: clusterLoggingConfig.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterLoggingConfig.ObjectMeta.Name + "_" + dest,
				Namespace: clusterLoggingConfig.ObjectMeta.Namespace,
			},
			Spec: model.ClusterLoggingConfigSpec{
				Type:            clusterLoggingConfig.Spec.Type,
				KubernetesPods:  clusterLoggingConfig.Spec.KubernetesPods,
				File:            clusterLoggingConfig.Spec.File,
				DestinationRefs: []string{dest},
			},
			Status: clusterLoggingConfig.Status,
		}

		applier := transformApplier{
			destination:   g.logDestinations[dest],
			labelFilter:   clusterLoggingConfig.Spec.LabelFilters,
			logFilter:     clusterLoggingConfig.Spec.LogFilters,
			multilineType: clusterLoggingConfig.Spec.MultiLineParser.Type,
		}

		transforms, err := applier.Do(sourceConfig.Spec.Transforms)
		if err != nil {
			return nil, err
		}

		sourceConfig.Spec.Transforms = transforms
		sources = append(sources, sourceConfig)
	}

	return sources, nil
}

func (g *Generator) clusterSources(input *go_hook.HookInput) ([]model.ClusterLoggingConfig, error) {
	snap := input.Snapshots["cluster_log_source"]
	clusterSources := make([]model.ClusterLoggingConfig, 0, len(snap))

	for _, s := range snap {
		clusterLoggingConfig := s.(v1alpha1.ClusterLoggingConfig)
		sources, err := g.convertClusterLoggingConfig(clusterLoggingConfig)
		if err != nil {
			return nil, err
		}
		clusterSources = append(clusterSources, sources...)
	}

	return clusterSources, nil
}

func (g *Generator) namespacedSources(input *go_hook.HookInput) ([]model.PodLoggingConfig, error) {
	snap := input.Snapshots["namespaced_log_source"]
	namespacedSources := make([]model.PodLoggingConfig, 0, len(snap))

	for _, s := range snap {
		podLoggingConfig := s.(v1alpha1.PodLoggingConfig)
		sources, err := g.convertPodLoggingConfig(podLoggingConfig)
		if err != nil {
			return nil, err
		}
		namespacedSources = append(namespacedSources, sources...)
	}

	return namespacedSources, nil
}
