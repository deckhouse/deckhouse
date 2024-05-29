/*
Copyright 2024 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// API

	Group        = "network.deckhouse.io"
	Version      = "v1alpha1"
	GroupVersion = Group + "/" + Version
	RTKind       = "RoutingTable"
	NRTKind      = "NodeRoutingTable"
	IRSKind      = "IPRuleSet"
	NIRSKind     = "NodeIPRuleSet"

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

type IPRuleSelectors struct {
	Not      bool     `json:"not,omitempty"`
	From     []string `json:"from,omitempty"`
	To       []string `json:"to,omitempty"`
	Tos      string   `json:"tos,omitempty"`
	FWMark   string   `json:"fwmark,omitempty"`
	IIf      string   `json:"iif,omitempty"`
	OIf      string   `json:"oif,omitempty"`
	UIDRange string   `json:"uidrange,omitempty"`
	IPProto  int      `json:"ipproto,omitempty"`
	SPort    string   `json:"sport,omitempty"`
	DPort    string   `json:"dport,omitempty"`
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

// CR NodeRoutingTable

type NodeRoutingTable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeRoutingTableSpec   `json:"spec"`
	Status            NodeRoutingTableStatus `json:"status,omitempty"`
}

type NodeRoutingTableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NodeRoutingTable `json:"items"`
}

type NodeRoutingTableSpec struct {
	NodeName         string  `json:"nodeName"`
	IPRoutingTableID int     `json:"ipRoutingTableID"`
	Routes           []Route `json:"routes"`
}

type NodeRoutingTableStatus struct {
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

// CR NodeIPRuleSet

type NodeIPRuleSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeIPRuleSetSpec   `json:"spec"`
	Status            NodeIPRuleSetStatus `json:"status,omitempty"`
}

type NodeIPRuleSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NodeIPRuleSet `json:"items"`
}

type NodeIPRuleSetSpec struct {
	NodeName string   `json:"nodeName"`
	IPRules  []IPRule `json:"rules"`
}

type NodeIPRuleSetStatus struct {
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	AppliedIPRules     []IPRule            `json:"appliedRules,omitempty"`
	Conditions         []ExtendedCondition `json:"conditions,omitempty"`
}
