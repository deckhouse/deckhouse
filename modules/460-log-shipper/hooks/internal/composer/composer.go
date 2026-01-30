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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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

// FromInput collects ClusterLoggingConfig sources from snapshots and combines them with provided destinations.
// Also records telemetry metrics for each custom resource found.
func FromInput(input *go_hook.HookInput, destinations []v1alpha1.ClusterLogDestination) (*Composer, error) {
	sourceSnap := input.Snapshots.Get("cluster_log_source")
	namespacedSourceSnap := input.Snapshots.Get("namespaced_log_source")

	res := &Composer{
		Source: make([]v1alpha1.ClusterLoggingConfig, 0, len(sourceSnap)+len(namespacedSourceSnap)),
		Dest:   make([]v1alpha1.ClusterLogDestination, 0, len(destinations)),
	}

	for _, dest := range destinations {
		res.Dest = append(res.Dest, dest)
		customResourceMetric(input, "ClusterLogDestination", dest.Name, dest.Namespace, dest.Spec.Type)
	}

	for src, err := range sdkobjectpatch.SnapshotIter[v1alpha1.ClusterLoggingConfig](sourceSnap) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over 'cluster_log_source' snapshots: %w", err)
		}
		res.Source = append(res.Source, src)
		customResourceMetric(input, "ClusterLoggingConfig", src.Name, src.Namespace, src.Spec.Type)
	}

	for src, err := range sdkobjectpatch.SnapshotIter[v1alpha1.PodLoggingConfig](namespacedSourceSnap) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over 'namespaced_log_source' snapshots: %w", err)
		}
		res.Source = append(res.Source, v1alpha1.NamespacedToCluster(src))
		customResourceMetric(input, "PodLoggingConfig", src.Name, src.Namespace, v1alpha1.SourceKubernetesPods)
	}

	return res, nil
}

func customResourceMetric(input *go_hook.HookInput, kind, name, namespace, _type string) {
	input.MetricsCollector.Set(telemetry.WrapName("log_shipper_custom_resource"), 1, map[string]string{
		"kind":         kind,
		"cr_name":      name,
		"cr_namespace": namespace,
		"cr_type":      _type,
	})
}

// Do composes the Vector configuration file from all sources and destinations.
func (c *Composer) Do() ([]byte, error) {
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

		destinations, err := c.composeDestinations(s.Spec.DestinationRefs, s.Spec.Type)
		if err != nil {
			return nil, err
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

func newLogSource(typ, name string, spec v1alpha1.ClusterLoggingConfigSpec) apis.LogSource {
	switch typ {
	case v1alpha1.SourceFile:
		return source.NewFile(name, spec.File)
	case v1alpha1.SourceKubernetesPods:
		return source.NewKubernetes(name, spec.KubernetesPods, false)
	}
	return nil
}

func (c *Composer) getDestinationSpecByName(name string) *v1alpha1.ClusterLogDestination {
	for _, d := range c.Dest {
		if d.Name == name {
			return &d
		}
	}
	return nil
}

// composeDestinations resolves destination references, creates destination instances, and applies transforms.
func (c *Composer) composeDestinations(destinationRefs []string, sourceType string) ([]PipelineDestination, error) {
	destinations := make([]PipelineDestination, 0, len(destinationRefs))

	for _, ref := range destinationRefs {
		// Create destination for this specific source to ensure correct labels
		destSpec := c.getDestinationSpecByName(ref)
		if destSpec == nil {
			continue
		}

		dest := newLogDest(destSpec.Spec.Type, destSpec.Name, destSpec.Spec, sourceType)
		if dest == nil {
			continue
		}

		transforms, err := transform.CreateLogDestinationTransforms(destSpec.Name, *destSpec, sourceType)
		if err != nil {
			return nil, err
		}

		destinations = append(destinations, PipelineDestination{
			Destination: dest,
			Transforms:  transforms,
		})
	}

	return destinations, nil
}

// newLogDest creates a log destination instance based on the destination type.
func newLogDest(typ, name string, spec v1alpha1.ClusterLogDestinationSpec, sourceType string) apis.LogDestination {
	switch typ {
	case v1alpha1.DestLoki:
		return destination.NewLoki(name, spec, sourceType)
	case v1alpha1.DestElasticsearch:
		return destination.NewElasticsearch(name, spec, sourceType)
	case v1alpha1.DestLogstash:
		return destination.NewLogstash(name, spec, sourceType)
	case v1alpha1.DestVector:
		return destination.NewVector(name, spec, sourceType)
	case v1alpha1.DestKafka:
		return destination.NewKafka(name, spec, sourceType)
	case v1alpha1.DestSplunk:
		return destination.NewSplunk(name, spec, sourceType)
	case v1alpha1.DestSocket:
		return destination.NewSocket(name, spec, sourceType)
	}
	return nil
}
