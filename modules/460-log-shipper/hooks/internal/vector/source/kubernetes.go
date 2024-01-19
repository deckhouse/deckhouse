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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

const defaultGlobCooldownMs = 1000

var _ apis.LogSource = (*Kubernetes)(nil)

// Kubernetes represents a source for collecting Kubernetes logs.
//
// Because of how selectors work in Kubernetes, it is not possible to declare OR selector.
// The following selector `metadata.namespace=ns1,metadata.namespace=ns2` selects nothing,
// because namespace cannot be ns1 and ns2 at the same time.
//
// Vector allows providing only a one node selector.
// Thus, the only way to collect logs from several namespaces is to render several `kubernetes_logs` sources for vector.
//
// Kubernetes handles this logic on building sources and generates the config according to a single deckhouse resource.
// (ClusterLoggingConfig or PodLoggingConfig)
type Kubernetes struct {
	commonSource

	namespaced bool // namespace or cluster Scope
	namespaces []string
	fields     []string

	labelSelector          string
	namespaceLabelSelector string

	annotationFields     KubernetesAnnotationFields
	nodeAnnotationFields NodeAnnotationFields
	globCooldownMs       int
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

type NodeAnnotationFields struct {
	NodeLabels string `json:"node_labels,omitempty"`
}

// rawKubernetesLogs represents `kubernetes_logs` vector source
// https://vector.dev/docs/reference/configuration/sources/kubernetes_logs/
type rawKubernetesLogs struct {
	commonSource

	Labels               string                     `json:"extra_label_selector,omitempty"`
	Fields               string                     `json:"extra_field_selector,omitempty"`
	NamespaceLabels      string                     `json:"extra_namespace_label_selector,omitempty"`
	AnnotationFields     KubernetesAnnotationFields `json:"annotation_fields,omitempty"`
	NodeAnnotationFields NodeAnnotationFields       `json:"node_annotation_fields,omitempty"`
	GlobCooldownMs       int                        `json:"glob_minimum_cooldown_ms,omitempty"`
	UserAPIServerCache   bool                       `json:"use_apiserver_cache,omitempty"`
}

func (k *rawKubernetesLogs) BuildSources() []apis.LogSource {
	return []apis.LogSource{k}
}

func NewKubernetes(name string, spec v1alpha1.KubernetesPodsSpec, namespaced bool) *Kubernetes {
	// Add a built-in filter to exclude logs by a label similar to vectore.dev/exclude=true
	// https://vector.dev/docs/reference/configuration/sources/kubernetes_logs/#pod-exclusion
	excludeSelector := metav1.LabelSelectorRequirement{
		Key:      "log-shipper.deckhouse.io/exclude",
		Operator: metav1.LabelSelectorOpNotIn,
		Values:   []string{"true"},
	}

	spec.LabelSelector.MatchExpressions = append(
		spec.LabelSelector.MatchExpressions, excludeSelector)
	spec.NamespaceSelector.LabelSelector.MatchExpressions = append(
		spec.NamespaceSelector.LabelSelector.MatchExpressions, excludeSelector)

	labelsSelector, err := metav1.LabelSelectorAsSelector(&spec.LabelSelector)
	if err != nil {
		// LabelSelector validated by OpenApi. Error in this place is very strange. We should panic.
		panic(err)
	}

	namespaceLabelsSelector, err := metav1.LabelSelectorAsSelector(&spec.NamespaceSelector.LabelSelector)
	if err != nil {
		// LabelSelector validated by OpenApi. Error in this place is very strange. We should panic.
		panic(err)
	}

	// Do not collect self logs because in case of en error vector starts overloading itself
	// by attempting to send error logs.
	fields := []string{"metadata.name!=$VECTOR_SELF_POD_NAME"}

	for _, ns := range spec.NamespaceSelector.ExcludeNames {
		fields = append(fields, "metadata.namespace!="+ns)
	}

	return &Kubernetes{
		commonSource: commonSource{
			Name: name,
			Type: "kubernetes_logs",
		},

		namespaced: namespaced,
		namespaces: spec.NamespaceSelector.MatchNames,
		fields:     fields,

		labelSelector:          labelsSelector.String(),
		namespaceLabelSelector: namespaceLabelsSelector.String(),
		annotationFields: KubernetesAnnotationFields{
			PodName:        "pod",
			PodLabels:      "pod_labels",
			PodIP:          "pod_ip",
			PodNamespace:   "namespace",
			ContainerImage: "image",
			ContainerName:  "container",
			PodNodeName:    "node",
			PodOwner:       "pod_owner",
		},
		nodeAnnotationFields: NodeAnnotationFields{
			NodeLabels: "node_labels",
		},
		globCooldownMs: defaultGlobCooldownMs,
	}
}

func (k *Kubernetes) newRawSource(name string, fields []string) *rawKubernetesLogs {
	return &rawKubernetesLogs{
		commonSource: commonSource{
			Type: k.Type,
			Name: name,
		},
		Fields:               strings.Join(fields, ","),
		Labels:               k.labelSelector,
		NamespaceLabels:      k.namespaceLabelSelector,
		AnnotationFields:     k.annotationFields,
		NodeAnnotationFields: k.nodeAnnotationFields,
		GlobCooldownMs:       k.globCooldownMs,
		UserAPIServerCache:   true,
	}
}

// BuildSources denormalizes sources for vector config, which can handle only one namespace per source
// (it is impossible to use OR clauses for the field-selector, so you can only select a single namespace)
func (k *Kubernetes) BuildSources() []apis.LogSource {
	if k.namespaced {
		ns := k.namespaces[0]
		return []apis.LogSource{k.newRawSource(
			"pod_logging_config/"+k.Name+"/"+ns,
			append([]string{"metadata.namespace=" + ns}, k.fields...),
		)}
	}

	if len(k.namespaces) == 0 {
		return []apis.LogSource{k.newRawSource(
			"cluster_logging_config/"+k.Name,
			k.fields,
		)}
	}

	res := make([]apis.LogSource, 0, len(k.namespaces))

	for _, ns := range k.namespaces {
		res = append(res, k.newRawSource(
			"cluster_logging_config/"+k.Name+":"+ns,
			append([]string{"metadata.namespace=" + ns}, k.fields...),
		))
	}

	return res
}
