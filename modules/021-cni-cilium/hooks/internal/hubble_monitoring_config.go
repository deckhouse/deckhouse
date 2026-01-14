/*
Copyright 2025 Flant JSC

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

package internal

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HubbleMonitoringConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HubbleMonitoringConfigSpec `json:"spec"`
}

type HubbleMonitoringConfigSpec struct {
	ExtendedMetrics ExtendedMetricsSpec `json:"extendedMetrics,omitempty"`
	FlowLogs        FlowLogsSpec        `json:"flowLogs,omitempty"`
}

type ExtendedMetricsSpec struct {
	Enabled    bool                      `json:"enabled,omitempty"`
	Collectors []ExtendedMetricCollector `json:"collectors,omitempty"`
}

type ExtendedMetricCollector struct {
	Name           string `json:"name,omitempty"`
	ContextOptions string `json:"contextOptions,omitempty"`
}

type FlowLogsSpec struct {
	Enabled         bool               `json:"enabled,omitempty"`
	AllowFilterList []*FlowLogFilter   `json:"allowFilterList,omitempty"`
	DenyFilterList  []*FlowLogFilter   `json:"denyFilterList,omitempty"`
	FieldMaskList   []FlowLogFieldMask `json:"fieldMaskList,omitempty"`
	FileMaxSizeMB   int32              `json:"fileMaxSizeMB,omitempty"`
}

type FlowLogFilter struct {
	UUID              []string         `json:"uuid,omitempty"`
	SourceIP          []string         `json:"source_ip,omitempty"`
	SourceIPXlated    []string         `json:"source_ip_xlated,omitempty"`
	SourcePod         []string         `json:"source_pod,omitempty"`
	SourceFQDN        []string         `json:"source_fqdn,omitempty"`
	SourceLabel       []string         `json:"source_label,omitempty"`
	SourceService     []string         `json:"source_service,omitempty"`
	SourceWorkload    []WorkloadFilter `json:"source_workload,omitempty"`
	SourceClusterName []string         `json:"source_cluster_name,omitempty"`

	DestinationIP          []string         `json:"destination_ip,omitempty"`
	DestinationPod         []string         `json:"destination_pod,omitempty"`
	DestinationFQDN        []string         `json:"destination_fqdn,omitempty"`
	DestinationLabel       []string         `json:"destination_label,omitempty"`
	DestinationService     []string         `json:"destination_service,omitempty"`
	DestinationWorkload    []WorkloadFilter `json:"destination_workload,omitempty"`
	DestinationClusterName []string         `json:"destination_cluster_name,omitempty"`

	TrafficDirection []string `json:"traffic_direction,omitempty"`
	Verdict          []string `json:"verdict,omitempty"`
	DropReasonDesc   []string `json:"drop_reason_desc,omitempty"`

	Interface []InterfaceFilter `json:"interface,omitempty"`
	EventType []EventTypeFilter `json:"event_type,omitempty"`

	HTTPStatusCode  []string `json:"http_status_code,omitempty"`
	Protocol        []string `json:"protocol,omitempty"`
	SourcePort      []string `json:"source_port,omitempty"`
	DestinationPort []string `json:"destination_port,omitempty"`
	DNSQuery        []string `json:"dns_query,omitempty"`

	SourceIdentity      []int32 `json:"source_identity,omitempty"`
	DestinationIdentity []int32 `json:"destination_identity,omitempty"`

	HTTPMethod []string           `json:"http_method,omitempty"`
	HTTPPath   []string           `json:"http_path,omitempty"`
	HTTPURL    []string           `json:"http_url,omitempty"`
	HTTPHeader []HTTPHeaderFilter `json:"http_header,omitempty"`

	TCPFlags []TCPFlagsFilter `json:"tcp_flags,omitempty"`

	NodeName   []string `json:"node_name,omitempty"`
	NodeLabels []string `json:"node_labels,omitempty"`

	IPVersion []string `json:"ip_version,omitempty"`
	TraceID   []string `json:"trace_id,omitempty"`
}

type WorkloadFilter struct {
	Name string `json:"name,omitempty"`
	Kind string `json:"kind,omitempty"`
}

type InterfaceFilter struct {
	Index int32  `json:"index,omitempty"`
	Name  string `json:"name,omitempty"`
}

type EventTypeFilter struct {
	Type         int32 `json:"type,omitempty"`
	MatchSubType bool  `json:"match_sub_type,omitempty"`
	SubType      int32 `json:"sub_type,omitempty"`
}

type HTTPHeaderFilter struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type TCPFlagsFilter struct {
	FIN bool `json:"FIN,omitempty"`
	SYN bool `json:"SYN,omitempty"`
	RST bool `json:"RST,omitempty"`
	PSH bool `json:"PSH,omitempty"`
	ACK bool `json:"ACK,omitempty"`
	URG bool `json:"URG,omitempty"`
	ECE bool `json:"ECE,omitempty"`
	CWR bool `json:"CWR,omitempty"`
	NS  bool `json:"NS,omitempty"`
}

type FlowLogFieldMask string

type HubbleMonitoringConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HubbleMonitoringConfig `json:"items"`
}
