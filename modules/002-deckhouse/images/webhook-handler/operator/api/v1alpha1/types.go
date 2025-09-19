package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
