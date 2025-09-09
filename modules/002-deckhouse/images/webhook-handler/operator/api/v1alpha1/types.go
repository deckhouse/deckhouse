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
	Name       string            `json:"name"`
	Kubernetes KubernetesContext `json:"kubernetes,omitempty"`
}

type KubernetesContext struct {
	Name                         string                `json:"name,omitempty"`
	WatchEventTypes              []WatchEventType      `json:"watchEvent,omitempty"`
	ExecuteHookOnEvents          []WatchEventType      `json:"executeHookOnEvent,omitempty"`
	ExecuteHookOnSynchronization bool                  `json:"executeHookOnSynchronization,omitempty"`
	WaitForSynchronization       string                `json:"waitForSynchronization,omitempty"`
	KeepFullObjectsInMemory      bool                  `json:"keepFullObjectsInMemory,omitempty"`
	Mode                         KubeEventMode         `json:"mode,omitempty"`
	ApiVersion                   string                `json:"apiVersion,omitempty"`
	Kind                         string                `json:"kind,omitempty"`
	NameSelector                 *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector                *metav1.LabelSelector `json:"labelSelector,omitempty"`
	FieldSelector                *FieldSelector        `json:"fieldSelector,omitempty"`
	Namespace                    *NamespaceSelector    `json:"namespace,omitempty"`
	JqFilter                     string                `json:"jqFilter,omitempty"`
	AllowFailure                 bool                  `json:"allowFailure,omitempty"`
	ResynchronizationPeriod      string                `json:"resynchronizationPeriod,omitempty"`
	IncludeSnapshotsFrom         []string              `json:"includeSnapshotsFrom,omitempty"`
	Queue                        string                `json:"queue,omitempty"`
	Group                        string                `json:"group,omitempty"`
}

type JqFilter struct {
	NodeName string `json:"nodeName"`
}
