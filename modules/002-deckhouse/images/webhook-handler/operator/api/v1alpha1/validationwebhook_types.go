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

	// +optional
	Foo *string `json:"foo,omitempty"`

	// ValidatingWebhook describes an webhook and the resources and operations it applies to.
	// +required
	ValidatingWebhook *KubernetesAdmissionConfigV1 `json:"validationObject,omitempty"`

	// +optional
	Context []Context `json:"context,omitempty"`

	// TODO: doc description
	// +required
	Handler Handler `json:"handler"`

	// status defines the observed state of ValidationWebhook
	// +optional
	Status ValidationWebhookStatus `json:"status,omitempty,omitzero"`
}

type Context struct {
	Name       string            `json:"name"`
	Kubernetes KubernetesContext `json:"kubernetes,omitempty"`
}

// type KubernetesContext struct {
// 	APIVersion        string                `json:"apiVersion,omitempty"`
// 	Kind              string                `json:"kind,omitempty"`
// 	NameSelector      *NameSelector         `json:"nameSelector,omitempty"`
// 	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitempty"`
// 	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
// 	JqFilter          JqFilter              `json:"jqFilter,omitempty"`
// }

type KubernetesContext struct {
	Name                         string                `json:"name,omitempty"`
	WatchEventTypes              []WatchEventType      `json:"watchEvent,omitempty"`
	ExecuteHookOnEvents          []WatchEventType      `json:"executeHookOnEvent,omitempty"`
	ExecuteHookOnSynchronization string                `json:"executeHookOnSynchronization,omitempty"`
	WaitForSynchronization       string                `json:"waitForSynchronization,omitempty"`
	KeepFullObjectsInMemory      string                `json:"keepFullObjectsInMemory,omitempty"`
	Mode                         KubeEventMode         `json:"mode,omitempty"`
	ApiVersion                   string                `json:"apiVersion,omitempty"`
	Kind                         string                `json:"kind,omitempty"`
	NameSelector                 *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector                *metav1.LabelSelector `json:"labelSelector,omitempty"`
	FieldSelector                *FieldSelector        `json:"fieldSelector,omitempty"`
	NamespaceSelector            *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	JqFilter                     string                `json:"jqFilter,omitempty"`
	AllowFailure                 bool                  `json:"allowFailure,omitempty"`
	ResynchronizationPeriod      string                `json:"resynchronizationPeriod,omitempty"`
	IncludeSnapshotsFrom         []string              `json:"includeSnapshotsFrom,omitempty"`
	Queue                        string                `json:"queue,omitempty"`
	Group                        string                `json:"group,omitempty"`
}

type WatchEventType string

const (
	WatchEventAdded    WatchEventType = "Added"
	WatchEventModified WatchEventType = "Modified"
	WatchEventDeleted  WatchEventType = "Deleted"
)

type KubeEventMode string

const (
	ModeV0          KubeEventMode = "v0"          // No first Synchronization, only Event.
	ModeIncremental KubeEventMode = "Incremental" // Send Synchronization with existed object and Event for each followed event.
)

type FieldSelectorRequirement struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}

type FieldSelector struct {
	MatchExpressions []FieldSelectorRequirement `json:"matchExpressions"`
}

type NameSelector struct {
	MatchNames []string `json:"matchNames"`
}

type JqFilter struct {
	NodeName string `json:"nodeName"`
}

type Handler struct {
	// this is a python script handler for object
	Python string `json:"python,omitempty"`

	// this is a cel rules handler for object
	// TODO: CEL support
	CEL string `json:"cel,omitempty"`
}

// version 1 of kubernetes validation configuration
type KubernetesAdmissionConfigV1 struct {
	Name                 string                  `json:"name,omitempty"`
	IncludeSnapshotsFrom []string                `json:"includeSnapshotsFrom,omitempty"`
	Group                string                  `json:"group,omitempty"`
	Rules                []v1.RuleWithOperations `json:"rules,omitempty"`
	FailurePolicy        *v1.FailurePolicyType   `json:"failurePolicy,omitempty"`
	LabelSelector        *metav1.LabelSelector   `json:"labelSelector,omitempty"`
	ObjectSelector       *metav1.LabelSelector   `json:"objectSelector,omitempty"`
	NamespaceSelector    *metav1.LabelSelector   `json:"namespaceSelector,omitempty"`
	SideEffects          *v1.SideEffectClass     `json:"sideEffects,omitempty"`
	TimeoutSeconds       *int32                  `json:"timeoutSeconds,omitempty"`
	MatchConditions      []v1.MatchCondition     `json:"matchConditions,omitempty"`
}

type NamespaceSelector struct {
	NameSelector  *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
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
