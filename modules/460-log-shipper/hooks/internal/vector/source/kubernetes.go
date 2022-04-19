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

package source

import (
	"fmt"
	"strings"

	"github.com/clarketm/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

// Kubernetes represents `kubernetes_logs` vector source
// https://vector.dev/docs/reference/configuration/sources/kubernetes_logs/
type Kubernetes struct {
	commonSource

	labels labels.Selector
	fields []string

	namespaced        bool // namespace or cluster Scope
	namespaces        []string
	excludeNamespaces []string

	annotationFields KubernetesAnnotationFields
}

// KubernetesAnnotationFields are supported fields for the following vector options
// https://vector.dev/docs/reference/configuration/sources/kubernetes_logs/#pod_annotation_fields
type KubernetesAnnotationFields struct {
	ContainerImage string `json:"container_image,omitempty"`
	ContainerName  string `json:"container_name,omitempty"`
	PodIP          string `json:"pod_ip,omitempty"`
	PodLabels      string `json:"pod_labels,omitempty"`
	PodName        string `json:"pod_name,omitempty"`
	PodNamespace   string `json:"pod_namespace,omitempty"`
	PodNodeName    string `json:"pod_node_name,omitempty"`
	PodOwner       string `json:"pod_owner,omitempty"`
}

func NewKubernetes(name string, spec v1alpha1.KubernetesPodsSpec, namespaced bool) impl.LogSource {
	labelsSelector, err := metav1.LabelSelectorAsSelector(&spec.LabelSelector)
	if err != nil {
		// LabelSelector validated by OpenApi. Error in this place is very strange. We should panic.
		panic(err)
	}

	formatFields := KubernetesAnnotationFields{
		PodName:        "pod",
		PodLabels:      "pod_labels",
		PodIP:          "pod_ip",
		PodNamespace:   "namespace",
		ContainerImage: "image",
		ContainerName:  "container",
		PodNodeName:    "node",
		PodOwner:       "pod_owner",
	}

	return Kubernetes{
		commonSource: commonSource{
			Name: name,
			Type: "kubernetes_logs",
		},
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
func (k Kubernetes) BuildSources() []impl.LogSource {
	if k.namespaced {
		k.Name = fmt.Sprintf("d8_namespaced_source_%s_%s", k.namespaces[0], k.Name)
		return []impl.LogSource{k}
	}

	if len(k.namespaces) <= 1 {
		k.Name = "d8_cluster_source_" + k.Name
		return []impl.LogSource{k}
	}

	res := make([]impl.LogSource, 0, len(k.namespaces))

	for _, ns := range k.namespaces {
		k := Kubernetes{
			commonSource:     commonSource{Name: fmt.Sprintf("d8_clusterns_source_%s_%s", ns, k.Name), Type: k.Type},
			namespaces:       []string{ns},
			labels:           k.labels,
			annotationFields: k.annotationFields,
		}

		res = append(res, k)
	}

	return res
}

func (k Kubernetes) MarshalJSON() ([]byte, error) {
	// Exclude pod logs to avoid fooling in case of problems and debugging.
	k.fields = append(k.fields, "metadata.name!=$VECTOR_SELF_POD_NAME")

	if len(k.namespaces) > 0 {
		ns := k.namespaces[0] // namespace should be denormalized here and have only one value
		k.fields = append(k.fields, "metadata.namespace="+ns)
	} else {
		// Apply namespaces exclusions only if the sync is not limited to a particular namespace.
		// This is validated by the CRD OpenAPI spec.
		for _, ns := range k.excludeNamespaces {
			k.fields = append(k.fields, "metadata.namespace!="+ns)
		}
	}

	s := struct {
		Type             string                     `json:"type"`
		Labels           string                     `json:"extra_label_selector,omitempty"`
		Fields           string                     `json:"extra_field_selector,omitempty"`
		AnnotationFields KubernetesAnnotationFields `json:"annotation_fields,omitempty"`
		GlobCooldownMs   int64                      `json:"glob_minimum_cooldown_ms,omitempty"`
	}{
		Type:             k.Type,
		Labels:           k.labels.String(),
		Fields:           strings.Join(k.fields, ","),
		AnnotationFields: k.annotationFields,
		GlobCooldownMs:   1000,
	}

	return json.Marshal(s)
}
