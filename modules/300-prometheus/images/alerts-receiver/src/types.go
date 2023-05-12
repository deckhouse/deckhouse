package main

import (
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterAlertFiring       = "Firing"
	clusterAlertFiringStaled = "Firing (staled)"
)

type ClusterAlert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Alert  ClusterAlertSpec   `json:"alert,omitempty"`
	Status ClusterAlertStatus `json:"status,omitempty"`
}

type ClusterAlertStatus struct {
	AlertStatus    string `json:"alertStatus,omitempty"`
	StartsAt       string `json:"startsAt,omitempty"`
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
}

type ClusterAlertSpec struct {
	Name          string         `json:"name"`
	SeverityLevel string         `json:"severity_level,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Description   string         `json:"description,omitempty"`
	Annotations   model.LabelSet `json:"annotations,omitempty"`
	Labels        model.LabelSet `json:"labels"`
}
