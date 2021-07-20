/*
Copyright 2021 Flant CJSC

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

package vector

import (
	"fmt"
	"strings"

	"github.com/clarketm/json"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

type commonSource struct {
	Name string `json:"-"`
	Type string `json:"type"`
}

func (cs commonSource) GetName() string {
	return cs.Name
}

type fileLogSource struct {
	commonSource

	Exclude   []string `json:"exclude,omitempty"`
	Include   []string `json:"include,omitempty"`
	Delimeter string   `json:"line_delimeter,omitempty"`
}

func NewFileLogSource(name string, spec v1alpha1.FileSpec) impl.LogSource {
	return fileLogSource{
		commonSource: commonSource{Name: name, Type: "file"},
		Exclude:      spec.Exclude,
		Include:      spec.Include,
		Delimeter:    spec.LineDelimiter,
	}
}

func (fs fileLogSource) BuildSources() []impl.LogSource {
	return []impl.LogSource{fs}
}

type kubernetesLogSource struct {
	commonSource

	labels []string
	fields []string

	namespaced bool // namespace or cluster Scope
	namespaces []string

	annotationFields kubeAnnotationFields
}

func NewKubernetesLogSource(name string, spec v1alpha1.KubernetesPodsSpec, namespaced bool) impl.LogSource {
	labels := make([]string, 0, len(spec.LabelSelector.MatchLabels))
	for k, v := range spec.LabelSelector.MatchLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}

	FormatFields := kubeAnnotationFields{
		PodName:        "pod",
		PodLabels:      "pod_labels",
		PodIP:          "pod_ip",
		PodNamespace:   "namespace",
		ContainerImage: "image",
		ContainerName:  "container",
		PodNodeName:    "node",
	}

	return kubernetesLogSource{
		commonSource:     commonSource{Name: name, Type: "kubernetes_logs"},
		namespaces:       spec.NamespaceSelector.MatchNames,
		labels:           labels,
		fields:           make([]string, 0),
		namespaced:       namespaced,
		annotationFields: FormatFields,
	}
}

// BuildSources denormalizes sources for vector config, which can handle only one namespace per source
// also mutates name of the source:
// 1. Namespaced - d8_namespaced_<ns>_<source_name>
// 2. Cluster - d8_cluster_<ns>_<source_name>
// 3. Cluster with NamespaceSelector - d8_clusterns_<ns>_<source_name>
func (cs kubernetesLogSource) BuildSources() []impl.LogSource {
	if cs.namespaced {
		cs.Name = fmt.Sprintf("d8_namespaced_%s_%s", cs.namespaces[0], cs.Name)
		return []impl.LogSource{cs}
	}

	if len(cs.namespaces) <= 1 {
		cs.Name = "d8_cluster_" + cs.Name
		return []impl.LogSource{cs}
	}

	res := make([]impl.LogSource, 0, len(cs.namespaces))

	for _, ns := range cs.namespaces {
		k := kubernetesLogSource{
			commonSource: commonSource{Name: fmt.Sprintf("d8_clusterns_%s_%s", ns, cs.Name), Type: cs.Type},
			namespaces:   []string{ns},
			labels:       cs.labels,
		}

		res = append(res, k)
	}

	return res
}

func (cs kubernetesLogSource) MarshalJSON() ([]byte, error) {
	if len(cs.namespaces) > 0 {
		ns := cs.namespaces[0] // namespace should be denormalized here and have only one value
		cs.fields = append(cs.fields, "metadata.namespace="+ns)
	}

	s := struct {
		Type             string               `json:"type"`
		Labels           string               `json:"extra_label_selector,omitempty"`
		Fields           string               `json:"extra_field_selector,omitempty"`
		AnnotationFields kubeAnnotationFields `json:"annotation_fields,omitempty"`
	}{
		Type:             cs.Type,
		Labels:           strings.Join(cs.labels, ","),
		Fields:           strings.Join(cs.fields, ","),
		AnnotationFields: cs.annotationFields,
	}

	return json.Marshal(s)
}
