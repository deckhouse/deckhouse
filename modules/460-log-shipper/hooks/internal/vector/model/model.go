/*
Copyright 2021 Flant JSC

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

package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

const (
	DestElasticsearch = "Elasticsearch"
	DestLogstash      = "Logstash"
	DestLoki          = "Loki"
)

const (
	SourceKubernetesPods = "KubernetesPods"
	SourceFile           = "File"
)

type ClusterLoggingConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a cluster log source.
	Spec ClusterLoggingConfigSpec `json:"spec"`

	// Most recently observed status of a cluster log source.
	// Populated by the system.
	Status v1alpha1.ClusterLoggingConfigStatus `json:"status,omitempty"`
}

type ClusterLoggingConfigSpec struct {
	// Type of cluster log source: KubernetesPods, File
	Type string `json:"type,omitempty"`

	// KubernetesPods describes spec for kubernetes pod source
	KubernetesPods v1alpha1.KubernetesPodsSpec `json:"kubernetesPods,omitempty"`

	// File describes spec for file source
	File v1alpha1.FileSpec `json:"file,omitempty"`

	// Transforms set ordered array of transforms. Possible values you can find into crd
	Transforms []impl.LogTransform `json:"transforms"`

	// DestinationRefs slice of ClusterLogDestination names
	DestinationRefs []string `json:"destinationRefs,omitempty"`
}

// PodLoggingConfig specify target for kubernetes pods logs collecting in specified namespace
type PodLoggingConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a namespaced log source.
	Spec PodLoggingConfigSpec `json:"spec"`

	// Most recently observed status of a namespaced log source.
	Status v1alpha1.PodLoggingConfigStatus `json:"status,omitempty"`
}

type PodLoggingConfigSpec struct {
	// LabelSelector filter pods by label
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Transforms set ordered array of transforms. Possible values you can find into crd
	Transforms []impl.LogTransform `json:"transforms"`

	// ClusterDestinationRefs slice of ClusterLogDestination names
	ClusterDestinationRefs []string `json:"clusterDestinationRefs,omitempty"`
}

type ElasticsearchEncoding struct {
	ExceptFields    []string `json:"except_fields,omitempty"`
	OnlyFields      []string `json:"only_fields,omitempty"`
	TimestampFormat string   `json:"timestamp_format,omitempty"`
}

type ElasticsearchAuth struct {
	Password      string `json:"password,omitempty"`
	Strategy      string `json:"strategy,omitempty"`
	User          string `json:"user,omitempty"`
	AwsAccessKey  string `json:"access_key_id,omitempty"`
	AwsSecretKey  string `json:"secret_access_key,omitempty"`
	AwsAssumeRole string `json:"assume_role,omitempty"`
}

type LogstashEncoding struct {
	ExceptFields    []string `json:"except_fields,omitempty"`
	OnlyFields      []string `json:"only_fields,omitempty"`
	Codec           string   `json:"codec,omitempty"`
	TimestampFormat string   `json:"timestamp_format,omitempty"`
}

type CommonTLS struct {
	CAFile         string `json:"ca_file,omitempty"`
	CertFile       string `json:"crt_file,omitempty"`
	KeyFile        string `json:"key_file,omitempty"`
	KeyPass        string `json:"key_pass,omitempty"`
	VerifyHostname bool   `json:"verify_hostname"`
}

type LogstashTLS struct {
	CommonTLS         `json:",inline"`
	VerifyCertificate bool `json:"verify_certificate"`
	Enabled           bool `json:"enabled"`
}

type LogstashKeepalive struct {
	TimeSecs int `json:"time_secs"`
}

type batch struct {
	MaxSize     uint32 `json:"max_bytes,omitempty"`
	TimeoutSecs uint32 `json:"timeout_secs,omitempty"`
}

type LokiEncoding struct {
	Codec           string   `json:"codec,omitempty"`
	OnlyFields      []string `json:"only_fields,omitempty"`
	TimestampFormat string   `json:"timestamp_format,omitempty"`
}

type kubeAnnotationFields struct {
	ContainerImage string `json:"container_image,omitempty"`
	ContainerName  string `json:"container_name,omitempty"`
	PodIP          string `json:"pod_ip,omitempty"`
	PodLabels      string `json:"pod_labels,omitempty"`
	PodName        string `json:"pod_name,omitempty"`
	PodNamespace   string `json:"pod_namespace,omitempty"`
	PodNodeName    string `json:"pod_node_name,omitempty"`
	PodOwner       string `json:"pod_owner,omitempty"`
}

type lokiDestination struct {
	commonDestinationSettings

	Encoding LokiEncoding `json:"encoding,omitempty"`

	Endpoint string `json:"endpoint"`

	Auth v1alpha1.LokiAuthSpec `json:"auth,omitempty"`

	TLS CommonTLS `json:"tls,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	RemoveLabelFields bool `json:"remove_label_fields"`

	OutOfOrderAction string `json:"out_of_order_action"`
}

type elasticsearchDestination struct {
	commonDestinationSettings

	Endpoint string `json:"endpoint"`

	Encoding ElasticsearchEncoding `json:"encoding,omitempty"`

	Batch batch `json:"batch,omitempty"`

	Auth ElasticsearchAuth `json:"auth,omitempty"`

	TLS CommonTLS `json:"tls,omitempty"`

	AWS region `json:"aws,omitempty"`

	Compression string `json:"compression,omitempty"`

	Index string `json:"index,omitempty"`

	Pipeline string `json:"pipeline,omitempty"`

	BulkAction string `json:"bulk_action,omitempty"`

	Mode string `json:"mode,omitempty"`

	DocType string `json:"doc_type,omitempty"`
}

type logstashDestination struct {
	commonDestinationSettings

	Address string `json:"address"`

	Encoding LogstashEncoding `json:"encoding,omitempty"`

	Mode string `json:"mode"`

	TLS LogstashTLS `json:"tls,omitempty"`

	Keepalive LogstashKeepalive `json:"keepalive,omitempty"`
}
