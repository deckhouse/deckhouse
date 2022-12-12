/*
Copyright 2022 Flant JSC

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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type OperationPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a node group.
	Spec OperationPolicySpec `json:"spec"`

	// Most recently observed status of the node.
	// Populated by the system.

	Status OperationPolicyStatus `json:"status,omitempty"`
}

type OperationPolicySpec struct {
	Policies struct {
		AllowedRepos      []string `json:"allowedRepos,omitempty"`
		RequiredResources struct {
			Limits   []string `json:"limits,omitempty"`
			Requests []string `json:"requests,omitempty"`
		} `json:"requiredResources,omitempty"`
		DisallowedImageTags []string `json:"disallowedImageTags,omitempty"`
		RequiredProbes      []string `json:"requiredProbes,omitempty"`
		RequiredLabels      []struct {
			Labels []struct {
				Key          string `json:"key,omitempty"`
				AllowedRegex string `json:"allowedRegex,omitempty"`
			} `json:"labels,omitempty"`
			WatchKinds []string `json:"watchKinds,omitempty"`
		} `json:"requiredLabels,omitempty"`
		MaxRevisionHistoryLimit   *int     `json:"maxRevisionHistoryLimit,omitempty"`
		ImagePullPolicy           string   `json:"imagePullPolicy,omitempty"`
		PriorityClassNames        []string `json:"priorityClassNames,omitempty"`
		CheckHostNetworkDNSPolicy bool     `json:"checkHostNetworkDNSPolicy,omitempty"`
		CheckContainerDuplicates  bool     `json:"checkContainerDuplicates,omitempty"`
	} `json:"policies"`
	Match struct {
		NamespaceSelector NamespaceSelector    `json:"namespaceSelector,omitempty"`
		LabelSelector     metav1.LabelSelector `json:"labelSelector,omitempty"`
	} `json:"match"`
}

type OperationPolicyStatus struct {
}

type NamespaceSelector struct {
	MatchNames   []string `json:"matchNames,omitempty"`
	ExcludeNames []string `json:"excludeNames,omitempty"`

	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}
