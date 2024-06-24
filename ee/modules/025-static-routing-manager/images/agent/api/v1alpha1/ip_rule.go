/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// network.deckhouse.io/v1alpha1

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
