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

package composer

import (
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/destination"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/source"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transform"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

type Composer struct {
	Source []v1alpha1.ClusterLoggingConfig
	Dest   []v1alpha1.ClusterLogDestination
}

func FromInput(input *go_hook.HookInput) *Composer {
	sourceSnap := input.Snapshots["cluster_log_source"]
	namespacedSourceSnap := input.Snapshots["namespaced_log_source"]
	destSnap := input.Snapshots["cluster_log_destination"]

	res := &Composer{
		Source: make([]v1alpha1.ClusterLoggingConfig, 0, len(sourceSnap)+len(namespacedSourceSnap)),
		Dest:   make([]v1alpha1.ClusterLogDestination, 0, len(destSnap)),
	}

	for _, d := range destSnap {
		dest := d.(v1alpha1.ClusterLogDestination)
		res.Dest = append(res.Dest, dest)
	}

	for _, s := range sourceSnap {
		src := s.(v1alpha1.ClusterLoggingConfig)
		res.Source = append(res.Source, src)
	}

	for _, ns := range namespacedSourceSnap {
		src := ns.(v1alpha1.PodLoggingConfig)
		res.Source = append(res.Source, v1alpha1.NamespacedToCluster(src))
	}

	return res
}

func (c *Composer) Do() ([]byte, error) {
	destinationRefs := c.composeDestinations()

	generator := NewLogConfigGenerator()

	for _, s := range c.Source {
		destinations := make([]PipelineDestination, 0, len(s.Spec.DestinationRefs))

		for _, ref := range s.Spec.DestinationRefs {
			destinations = append(destinations, destinationRefs["d8_cluster_sink_"+ref])
		}

		transforms, err := transform.CreateLogSourceTransforms(&transform.LogSourceConfig{
			SourceType:    s.Spec.Type,
			MultilineType: s.Spec.MultiLineParser.Type,
			LabelFilter:   s.Spec.LabelFilters,
			LogFilter:     s.Spec.LabelFilters,
		})

		if err != nil {
			return nil, err
		}

		generator.AppendLogPipeline(&Pipeline{
			Source:       newLogSource(s.Spec.Type, s.Name, s.Spec),
			Transforms:   transforms,
			Destinations: destinations,
		})
	}

	return generator.GenerateConfig()
}

func (c *Composer) composeDestinations() map[string]PipelineDestination {
	destinationByName := make(map[string]PipelineDestination)

	for _, d := range c.Dest {
		dest := newLogDest(d.Spec.Type, d.Name, d.Spec)

		destinationByName[dest.GetName()] = PipelineDestination{
			Destination: dest,
			Transforms:  transform.CreateLogDestinationTransforms(d),
		}
	}

	return destinationByName
}

func newLogSource(typ, name string, spec v1alpha1.ClusterLoggingConfigSpec) apis.LogSource {
	switch typ {
	case v1alpha1.SourceFile:
		return source.NewFile(name, spec.File)
	case v1alpha1.SourceKubernetesPods:
		return source.NewKubernetes(name, spec.KubernetesPods, false)
	}
	return nil
}

func newLogDest(typ, name string, spec v1alpha1.ClusterLogDestinationSpec) apis.LogDestination {
	switch typ {
	case v1alpha1.DestLoki:
		return destination.NewLoki(name, spec)
	case v1alpha1.DestElasticsearch:
		return destination.NewElasticsearch(name, spec)
	case v1alpha1.DestLogstash:
		return destination.NewLogstash(name, spec)
	case v1alpha1.DestVector:
		return destination.NewVector(name, spec)
	}
	return nil
}
