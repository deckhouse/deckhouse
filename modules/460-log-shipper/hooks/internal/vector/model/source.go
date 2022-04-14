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

package model

import (
	"fmt"
	"strings"

	"github.com/clarketm/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

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
	Delimiter string   `json:"line_delimiter,omitempty"`
}

func NewFileLogSource(name string, spec v1alpha1.FileSpec) impl.LogSource {
	return fileLogSource{
		commonSource: commonSource{Name: name, Type: "file"},
		Exclude:      spec.Exclude,
		Include:      spec.Include,
		Delimiter:    spec.LineDelimiter,
	}
}

func (fs fileLogSource) BuildSources() []impl.LogSource {
	return []impl.LogSource{fs}
}

type kubernetesLogSource struct {
	commonSource

	labels labels.Selector
	fields []string

	namespaced        bool // namespace or cluster Scope
	namespaces        []string
	excludeNamespaces []string

	annotationFields kubeAnnotationFields
}

func NewKubernetesLogSource(name string, spec v1alpha1.KubernetesPodsSpec, namespaced bool) impl.LogSource {
	labelsSelector, err := metav1.LabelSelectorAsSelector(&spec.LabelSelector)
	if err != nil {
		// LabelSelector validated by OpenApi. Error in this place is very strange. We should panic.
		panic(err)
	}

	formatFields := kubeAnnotationFields{
		PodName:        "pod",
		PodLabels:      "pod_labels",
		PodIP:          "pod_ip",
		PodNamespace:   "namespace",
		ContainerImage: "image",
		ContainerName:  "container",
		PodNodeName:    "node",
		PodOwner:       "pod_owner",
	}

	return kubernetesLogSource{
		commonSource:      commonSource{Name: name, Type: "kubernetes_logs"},
		namespaces:        spec.NamespaceSelector.MatchNames,
		excludeNamespaces: spec.NamespaceSelector.ExcludeNames,
		labels:            labelsSelector,
		fields:            make([]string, 0),
		namespaced:        namespaced,
		annotationFields:  formatFields,
	}
}

// BuildSources denormalizes sources for vector config, which can handle only one namespace per source
// (it is impossible to use OR clauses for the field-selector, so you can only select a single namespace)
//
// Also mutates name of the source:
// 1. Namespaced - d8_namespaced_<ns>_<source_name>
// 2. Cluster - d8_cluster_<ns>_<source_name>
// 3. Cluster with NamespaceSelector - d8_clusterns_<ns>_<source_name>
func (cs kubernetesLogSource) BuildSources() []impl.LogSource {
	if cs.namespaced {
		cs.Name = fmt.Sprintf("d8_namespaced_source_%s_%s", cs.namespaces[0], cs.Name)
		return []impl.LogSource{cs}
	}

	if len(cs.namespaces) <= 1 {
		cs.Name = "d8_cluster_source_" + cs.Name
		return []impl.LogSource{cs}
	}

	res := make([]impl.LogSource, 0, len(cs.namespaces))

	for _, ns := range cs.namespaces {
		k := kubernetesLogSource{
			commonSource:     commonSource{Name: fmt.Sprintf("d8_clusterns_source_%s_%s", ns, cs.Name), Type: cs.Type},
			namespaces:       []string{ns},
			labels:           cs.labels,
			annotationFields: cs.annotationFields,
		}

		res = append(res, k)
	}

	return res
}

func (cs kubernetesLogSource) MarshalJSON() ([]byte, error) {
	cs.fields = append(cs.fields, "metadata.name!=$VECTOR_SELF_POD_NAME")

	if len(cs.namespaces) > 0 {
		ns := cs.namespaces[0] // namespace should be denormalized here and have only one value
		cs.fields = append(cs.fields, "metadata.namespace="+ns)
	} else {
		// Apply namespaces exclusions only if the sync is not limited to a particular namespace.
		// This is validated by the CRD OpenAPI spec.
		for _, ns := range cs.excludeNamespaces {
			cs.fields = append(cs.fields, "metadata.namespace!="+ns)
		}
	}

	s := struct {
		Type             string               `json:"type"`
		Labels           string               `json:"extra_label_selector,omitempty"`
		Fields           string               `json:"extra_field_selector,omitempty"`
		AnnotationFields kubeAnnotationFields `json:"annotation_fields,omitempty"`
		GlobCooldownMs   int64                `json:"glob_minimum_cooldown_ms,omitempty"`
	}{
		Type:             cs.Type,
		Labels:           cs.labels.String(),
		Fields:           strings.Join(cs.fields, ","),
		AnnotationFields: cs.annotationFields,
		GlobCooldownMs:   1000,
	}

	return json.Marshal(s)
}
