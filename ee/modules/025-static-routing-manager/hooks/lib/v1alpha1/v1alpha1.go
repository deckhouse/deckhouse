/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// API

	Group                = "network.deckhouse.io"
	InternalGroup        = "internal.network.deckhouse.io"
	Version              = "v1alpha1"
	GroupVersion         = Group + "/" + Version
	InternalGroupVersion = InternalGroup + "/" + Version
	RTKind               = "RoutingTable"
	NRTKind              = "SDNInternalNodeRoutingTable"
	IRSKind              = "IPRuleSet"
	NIRSKind             = "SDNInternalNodeIPRuleSet"

	// Labels, annotations, finalizers

	Finalizer = "routing-tables-manager.network.deckhouse.io"

	// Types

	ReconciliationSucceedType = "Ready"

	// Reasons

	ReconciliationReasonSucceed = "ReconciliationSucceed"
	ReconciliationReasonFailed  = "ReconciliationFailed"
	ReconciliationReasonPending = "Pending"
)

// network.deckhouse.io/v1alpha1

type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
}

type Routes struct {
	Routes []Route `json:"routes"`
}

type IPRule struct {
	Priority  int             `json:"priority,omitempty"`
	Selectors IPRuleSelectors `json:"selectors"`
	Actions   IPRuleActions   `json:"actions"`
}

type UIDRange struct {
	Start uint32 `json:"start,omitempty"`
	End   uint32 `json:"end,omitempty"`
}

type PortRange struct {
	Start uint16 `json:"start,omitempty"`
	End   uint16 `json:"end,omitempty"`
}

type IPRuleSelectors struct {
	Not        bool      `json:"not,omitempty"`
	From       []string  `json:"from,omitempty"`
	To         []string  `json:"to,omitempty"`
	Tos        string    `json:"tos,omitempty"`
	FWMark     string    `json:"fwMark,omitempty"`
	IIf        string    `json:"iif,omitempty"`
	OIf        string    `json:"oif,omitempty"`
	UIDRange   UIDRange  `json:"uidRange,omitempty"`
	IPProto    int       `json:"ipProto,omitempty"`
	SPortRange PortRange `json:"sportRange,omitempty"`
	DPortRange PortRange `json:"dportRange,omitempty"`
}

type IPRuleActions struct {
	Lookup IPRuleActionsLookup `json:"lookup,omitempty"`
}

type IPRuleActionsLookup struct {
	IPRoutingTableID int    `json:"ipRoutingTableID,omitempty"`
	RoutingTableName string `json:"routingTableName,omitempty"`
}

type ExtendedCondition struct {
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty"`
	metav1.Condition  `json:",inline"`
}

// CR RoutingTable

type RoutingTable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RoutingTableSpec   `json:"spec"`
	Status            RoutingTableStatus `json:"status,omitempty"`
}

type RoutingTableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RoutingTable `json:"items"`
}

type RoutingTableSpec struct {
	IPRoutingTableID int               `json:"ipRoutingTableID"`
	Routes           []Route           `json:"routes"`
	NodeSelector     map[string]string `json:"nodeSelector"`
}

type RoutingTableStatus struct {
	ObservedGeneration        int64               `json:"observedGeneration,omitempty"`
	IPRoutingTableID          int                 `json:"ipRoutingTableID,omitempty"`
	ReadyNodeRoutingTables    int                 `json:"readyNodeRoutingTables,omitempty"`
	AffectedNodeRoutingTables int                 `json:"affectedNodeRoutingTables,omitempty"`
	Conditions                []ExtendedCondition `json:"conditions,omitempty"`
}

// CR SDNInternalNodeRoutingTable

type SDNInternalNodeRoutingTable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SDNInternalNodeRoutingTableSpec   `json:"spec"`
	Status            SDNInternalNodeRoutingTableStatus `json:"status,omitempty"`
}

type SDNInternalNodeRoutingTableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SDNInternalNodeRoutingTable `json:"items"`
}

type SDNInternalNodeRoutingTableSpec struct {
	NodeName         string  `json:"nodeName"`
	IPRoutingTableID int     `json:"ipRoutingTableID"`
	Routes           []Route `json:"routes"`
}

type SDNInternalNodeRoutingTableStatus struct {
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	AppliedRoutes      []Route             `json:"appliedRoutes,omitempty"`
	Conditions         []ExtendedCondition `json:"conditions,omitempty"`
}

// CR IPRuleSet

type IPRuleSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              IPRuleSetSpec   `json:"spec"`
	Status            IPRuleSetStatus `json:"status,omitempty"`
}

type IPRuleSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []IPRuleSet `json:"items"`
}

type IPRuleSetSpec struct {
	IPRules      []IPRule          `json:"rules"`
	NodeSelector map[string]string `json:"nodeSelector"`
}

type IPRuleSetStatus struct {
	ObservedGeneration     int64               `json:"observedGeneration,omitempty"`
	ReadyNodeIPRuleSets    int                 `json:"readyNodeIPRuleSets,omitempty"`
	AffectedNodeIPRuleSets int                 `json:"affectedNodeIPRuleSets,omitempty"`
	Conditions             []ExtendedCondition `json:"conditions,omitempty"`
}

// CR SDNInternalNodeIPRuleSet

type SDNInternalNodeIPRuleSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SDNInternalNodeIPRuleSetSpec   `json:"spec"`
	Status            SDNInternalNodeIPRuleSetStatus `json:"status,omitempty"`
}

type SDNInternalNodeIPRuleSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SDNInternalNodeIPRuleSet `json:"items"`
}

type SDNInternalNodeIPRuleSetSpec struct {
	NodeName string   `json:"nodeName"`
	IPRules  []IPRule `json:"rules"`
}

type SDNInternalNodeIPRuleSetStatus struct {
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	AppliedIPRules     []IPRule            `json:"appliedRules,omitempty"`
	Conditions         []ExtendedCondition `json:"conditions,omitempty"`
}
