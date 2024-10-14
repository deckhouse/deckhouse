/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	D8Realm = 216 // d8 hex = 216 dec

	// Labels, annotations, finalizers

	NodeNameLabel = "routing-manager.network.deckhouse.io/node-name"
	Finalizer     = "routing-tables-manager.network.deckhouse.io"

	// Types

	ReconciliationSucceedType = "Ready"

	// Reasons

	ReconciliationReasonSucceed = "ReconciliationSucceed"
	ReconciliationReasonFailed  = "ReconciliationFailed"
	ReconciliationReasonPending = "Pending"
)

// network.deckhouse.io/v1alpha1

type ExtendedCondition struct {
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`
	metav1.Condition  `json:",inline"`
}
