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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/telemetry"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/destination"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/source"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vector/transform"
)

type Composer struct {
	Source []v1alpha1.ClusterLoggingConfig
	Dest   []v1alpha1.ClusterLogDestination
}

func FromInput(input *go_hook.HookInput, destinations []v1alpha1.ClusterLogDestination) *Composer {
	sourceSnap := input.Snapshots["cluster_log_source"]
	namespacedSourceSnap := input.Snapshots["namespaced_log_source"]

	res := &Composer{
		Source: make([]v1alpha1.ClusterLoggingConfig, 0, len(sourceSnap)+len(namespacedSourceSnap)),
		Dest:   make([]v1alpha1.ClusterLogDestination, 0, len(destinations)),
	}

	for _, dest := range destinations {
		res.Dest = append(res.Dest, dest)
		customResourceMetric(input, "ClusterLogDestination", dest.Name, dest.Namespace, dest.Spec.Type)
	}

	for _, s := range sourceSnap {
		src := s.(v1alpha1.ClusterLoggingConfig)
		res.Source = append(res.Source, src)
		customResourceMetric(input, "ClusterLoggingConfig", src.Name, src.Namespace, src.Spec.Type)
	}

	for _, ns := range namespacedSourceSnap {
		src := ns.(v1alpha1.PodLoggingConfig)
		res.Source = append(res.Source, v1alpha1.NamespacedToCluster(src))
		customResourceMetric(input, "PodLoggingConfig", src.Name, src.Namespace, v1alpha1.SourceKubernetesPods)
	}

	return res
}

func customResourceMetric(input *go_hook.HookInput, kind, name, namespace, _type string) {
	input.MetricsCollector.Set(telemetry.WrapName("log_shipper_custom_resource"), 1, map[string]string{
		"kind":         kind,
		"cr_name":      name,
		"cr_namespace": namespace,
		"cr_type":      _type,
	})
}

func (c *Composer) Do() ([]byte, error) {
	destinationRefs, err := c.composeDestinations()
	if err != nil {
		return nil, err
	}

	file := NewVectorFile()

	for _, s := range c.Source {
		transforms, err := transform.CreateLogSourceTransforms(s.Name, &transform.LogSourceConfig{
			SourceType:            s.Spec.Type,
			MultilineType:         s.Spec.MultiLineParser.Type,
			MultilineCustomConfig: s.Spec.MultiLineParser.Custom,
			LabelFilter:           s.Spec.LabelFilters,
			LogFilter:             s.Spec.LogFilters,
		})
		if err != nil {
			return nil, err
		}

		src := PipelineSource{
			Source:     newLogSource(s.Spec.Type, s.Name, s.Spec),
			Transforms: transforms,
		}

		var destinations []PipelineDestination

		for _, ref := range s.Spec.DestinationRefs {
			dst := destinationRefs[destination.ComposeName(ref)]

			if dst.Destination != nil {
				destinations = append(destinations, dst)
			}
		}

		if len(destinations) > 0 {
			err = file.AppendLogPipeline(&Pipeline{
				Source:       src,
				Destinations: destinations,
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return file.ConvertToJSON()
}

func (c *Composer) composeDestinations() (map[string]PipelineDestination, error) {
	destinationByName := make(map[string]PipelineDestination)

	for _, d := range c.Dest {
		dest := newLogDest(d.Spec.Type, d.Name, d.Spec)

		transforms, err := transform.CreateLogDestinationTransforms(d.Name, d)
		if err != nil {
			return nil, err
		}

		destinationByName[dest.GetName()] = PipelineDestination{
			Destination: dest,
			Transforms:  transforms,
		}
	}

	return destinationByName, nil
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
	case v1alpha1.DestKafka:
		return destination.NewKafka(name, spec)
	case v1alpha1.DestSplunk:
		return destination.NewSplunk(name, spec)
	case v1alpha1.DestSocket:
		return destination.NewSocket(name, spec)
	}
	return nil
}
