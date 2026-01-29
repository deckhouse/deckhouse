// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

var (
	ValidationWebhookFinalizer = "validationwebhooks.deckhouse.io/finalizer"
	ConversionWebhookFinalizer = "conversionwebhooks.deckhouse.io/finalizer"
)

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

type NamespaceSelector struct {
	NameSelector  *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type Context struct {
	// It is used to distinguish different bindings during runtime.
	Name       string            `json:"name"`
	Kubernetes KubernetesContext `json:"kubernetes,omitempty"`
}

type KubernetesContext struct {
	WatchEventTypes []WatchEventType `json:"watchEvent,omitempty"`
	// The list of events which led to a hook's execution.
	// By default, all events are used to execute a hook: "Added", "Modified" and "Deleted".
	ExecuteHookOnEvents []WatchEventType `json:"executeHookOnEvent,omitempty"`
	// If `false`, Shell-operator skips the hook execution with Synchronization binding context.
	ExecuteHookOnSynchronization bool   `json:"executeHookOnSynchronization,omitempty"`
	WaitForSynchronization       string `json:"waitForSynchronization,omitempty"`
	// If not set or `true`, dumps of Kubernetes resources are cached
	// for this binding, and the snapshot includes them as object fields.
	// Set to `false` if the hook does not rely on full objects to reduce the memory footprint.
	KeepFullObjectsInMemory bool          `json:"keepFullObjectsInMemory,omitempty"`
	Mode                    KubeEventMode `json:"mode,omitempty"`
	// Is an optional group and version of object API.
	// For example, it is `v1` for core objects (Pod, etc.), `rbac.authorization.k8s.io/v1beta1` for ClusterRole and `monitoring.coreos.com/v1` for prometheus-operator.
	ApiVersion string `json:"apiVersion,omitempty"`
	// Is the type of a monitored Kubernetes resource. This field is required.
	Kind          string                `json:"kind"`
	NameSelector  *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	FieldSelector *FieldSelector        `json:"fieldSelector,omitempty"`
	// Filters to choose namespaces.
	Namespace *NamespaceSelector `json:"namespace,omitempty"`
	// An optional parameter that specifies event filtering using jq syntax.
	// The hook will be triggered on the "Modified" event only if the filter result is changed after the last event.
	JqFilter string `json:"jqFilter,omitempty"`
	// If `true`, Shell-operator skips the hook execution errors.
	// If `false` or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.
	AllowFailure            bool   `json:"allowFailure,omitempty"`
	ResynchronizationPeriod string `json:"resynchronizationPeriod,omitempty"`
	// An array of names of kubernetes bindings in a hook.
	// When specified, a list of monitored objects from that bindings
	// will be added to the binding context in a snapshots field. Self-include is also possible.
	IncludeSnapshotsFrom []string `json:"includeSnapshotsFrom,omitempty"`
	// A name of a separate queue. It can be used to execute long-running hooks in parallel with hooks in the "main" queue.
	Queue string `json:"queue,omitempty"`
	// A key to include snapshots from a group of schedule and kubernetes bindings.
	Group string `json:"group,omitempty"`
}

type JqFilter struct {
	NodeName string `json:"nodeName"`
}
