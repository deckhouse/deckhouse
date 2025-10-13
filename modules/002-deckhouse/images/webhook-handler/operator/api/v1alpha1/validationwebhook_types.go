/*
Copyright 2025.

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

import (
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make" to regenerate code after modifying this file
// NOTE: json tags are required.

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=vwhc

// ValidationWebhook is the Schema for the validationwebhooks API
type ValidationWebhook struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// ValidatingWebhook describes an webhook and the resources and operations it applies to.
	// +required
	ValidatingWebhook *KubernetesAdmissionConfigV1 `json:"validationObject,omitempty"`

	// Run a hook on a Kubernetes object changes.
	// +optional
	Context []Context `json:"context,omitempty"`

	// Code of the ValidatingWebhook handler
	// +required
	Handler ValidationWebhookHandler `json:"handler"`

	// status defines the observed state of ValidationWebhook
	// +optional
	Status ValidationWebhookStatus `json:"status,omitempty,omitzero"`
}

type ValidationWebhookHandler struct {
	// this is a python script handler for object
	Python string `json:"python,omitempty"`

	// this is a cel rules handler for object
	// TODO: CEL support
	CEL string `json:"cel,omitempty"`
}

// version 1 of kubernetes validation configuration
type KubernetesAdmissionConfigV1 struct {
	//  should be a domain with at least three segments separated by dots.
	Name string `json:"name"`
	// an array of names of kubernetes bindings in a hook. When specified, a list of monitored objects from these bindings will be added to the binding context in the snapshots field.
	IncludeSnapshotsFrom []string `json:"includeSnapshotsFrom,omitempty"`
	// a key to include snapshots from a group of schedule and kubernetes bindings. See grouping.
	Group string `json:"group,omitempty"`
	// a required list of rules used to determine if a request to the Kubernetes API server should be sent to the hook.
	Rules []v1.RuleWithOperations `json:"rules,omitempty"`
	// defines how errors from the hook are handled.
	FailurePolicy *v1.FailurePolicyType `json:"failurePolicy,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	Namespace     *NamespaceSelector    `json:"namespace,omitempty"`
	// determine whether the hook is dryRun-aware.
	SideEffects *v1.SideEffectClass `json:"sideEffects,omitempty"`
	// a seconds API server should wait for a hook to respond before treating the call as a failure. Default is 10 (seconds).
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
	// an optional list of match conditions for fine-grained request filtering. Available only since v1.27 of Kubernetes.
	MatchConditions []v1.MatchCondition `json:"matchConditions,omitempty"`
}

// ValidationWebhookStatus defines the observed state of ValidationWebhook.
type ValidationWebhookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
}

// +kubebuilder:object:root=true

// ValidationWebhookList contains a list of ValidationWebhook
type ValidationWebhookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValidationWebhook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ValidationWebhook{}, &ValidationWebhookList{})
}
