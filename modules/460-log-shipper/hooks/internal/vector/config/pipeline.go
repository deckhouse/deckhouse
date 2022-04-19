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

package config

import (
	"fmt"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/model"
)

func newLogSource(typ, name string, spec model.ClusterLoggingConfigSpec) impl.LogSource {
	switch typ {
	case model.SourceFile:
		return model.NewFileLogSource(name, spec.File)
	case model.SourceKubernetesPods:
		return model.NewKubernetesLogSource(name, spec.KubernetesPods, false)
	}
	return nil
}

func newLogDest(typ, name string, spec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	switch typ {
	case model.DestLoki:
		return model.NewLokiDestination(name, spec)
	case model.DestElasticsearch:
		return model.NewElasticsearchDestination(name, spec)
	case model.DestLogstash:
		return model.NewLogstashDestination(name, spec)
	}
	return nil
}

// Pipeline is a representation of a single logical tube.
//   Example: ClusterLoggingConfig +(destinationRef) ClusterLogsDestination = Single Pipeline.
type Pipeline struct {
	Source       impl.LogSource
	Transforms   []impl.LogTransform
	Destinations []impl.LogDestination
}

// NewPipeline creates an empty pipeline instance.
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// NewPipelineNamespaced creates a new pipeline instance for the namespaced PodLoggingConfig resource.
func NewPipelineNamespaced(generator *LogConfigGenerator, destMap map[string]v1alpha1.ClusterLogDestination, sourceConfig *model.PodLoggingConfig) (*Pipeline, error) {
	pipeline := NewPipeline()

	var err error

	// for each source looking for all cluster destinations
	for _, dstRef := range sourceConfig.Spec.ClusterDestinationRefs {
		cdest, ok := destMap[dstRef]
		if !ok {
			return nil, fmt.Errorf("clusterDestinationRef: %s for PodLoggingConfig: %s not found, skipping", dstRef, sourceConfig.Name)
		}

		dest := newLogDest(cdest.Spec.Type, cdest.Name, cdest.Spec)
		pipeline.Destinations = append(pipeline.Destinations, dest)
	}

	namespacedName := fmt.Sprintf("%s_%s", sourceConfig.Namespace, sourceConfig.Name)
	// prefer snippet over structured transforms
	if len(sourceConfig.Spec.Transforms) > 0 {
		pipeline.Transforms, err = generator.BuildTransforms(namespacedName, sourceConfig.Spec.Transforms)
		if err != nil {
			return nil, fmt.Errorf("%w, transforms build from snippet failed, skipping", err)
		}
	}

	// set namespace selector to config namespace. It's only 1 namespace available for Namespaced config
	kubeSpec := v1alpha1.KubernetesPodsSpec{
		NamespaceSelector: v1alpha1.NamespaceSelector{MatchNames: []string{sourceConfig.Namespace}},
		LabelSelector:     sourceConfig.Spec.LabelSelector,
	}
	pipeline.Source = model.NewKubernetesLogSource(sourceConfig.Name, kubeSpec, true)

	return pipeline, nil
}

// NewPipelineCluster creates a new pipeline instance for the cluster scoped ClusterLoggingConfig resource.
func NewPipelineCluster(generator *LogConfigGenerator, destMap map[string]v1alpha1.ClusterLogDestination, sourceConfig *model.ClusterLoggingConfig) (*Pipeline, error) {
	pipeline := NewPipeline()

	var err error

	// for each source looking for all destinations
	for _, dstRef := range sourceConfig.Spec.DestinationRefs {
		cdest, ok := destMap[dstRef]
		if !ok {
			return nil, fmt.Errorf("destinationRef: %s for ClusterLoggingConfig: %s not found, skipping", dstRef, sourceConfig.Name)

		}
		dest := newLogDest(cdest.Spec.Type, cdest.Name, cdest.Spec)
		pipeline.Destinations = append(pipeline.Destinations, dest)
	}

	if len(sourceConfig.Spec.Transforms) > 0 {
		pipeline.Transforms, err = generator.BuildTransforms(sourceConfig.Name, sourceConfig.Spec.Transforms)
		if err != nil {
			return nil, fmt.Errorf("%w, transforms build from snippet failed, skipping", err)
		}
	}

	pipeline.Source = newLogSource(sourceConfig.Spec.Type, sourceConfig.Name, sourceConfig.Spec)

	return pipeline, nil
}
