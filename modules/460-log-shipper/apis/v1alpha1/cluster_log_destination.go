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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterLogDestination specify output for logs stream
type ClusterLogDestination struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a cluster log source.
	Spec ClusterLogDestinationSpec `json:"spec"`

	// Most recently observed status of a cluster log source.
	// Populated by the system.
	Status ClusterLogDestinationStatus `json:"status,omitempty"`
}

type ClusterLogDestinationSpec struct {
	// Type of cluster log source: Loki, Elasticsearch, Logstash, Vector
	Type string `json:"type,omitempty"`

	// Loki describes spec for loki endpoint
	Loki LokiSpec `json:"loki"`

	// Elasticsearch spec for the Elasticsearch endpoint
	Elasticsearch ElasticsearchSpec `json:"elasticsearch"`

	// Logstash spec for the Logstash endpoint
	Logstash LogstashSpec `json:"logstash"`

	// Kafka spec for the Kafka endpoint
	Kafka KafkaSpec `json:"kafka"`

	// Splunk spec for the Splunk endpoint
	Splunk SplunkSpec `json:"splunk"`

	// Socket spec for the Socket endpoint
	Socket SocketSpec `json:"socket"`

	// Vector spec for the Vector endpoint
	Vector VectorSpec `json:"vector"`

	// Add extra labels for sources
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	// Add rateLimit for sink
	RateLimit RateLimitSpec `json:"rateLimit,omitempty"`

	Buffer *Buffer `json:"buffer,omitempty"`
}

type ClusterLogDestinationStatus struct {
}

type LokiAuthSpec struct {
	Password string `json:"password,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Token    string `json:"token,omitempty"`
	User     string `json:"user,omitempty"`
}

// RateLimitSpec is throttle-transform configuration.
type RateLimitSpec struct {
	LinesPerMinute *int32   `json:"linesPerMinute,omitempty"`
	KeyField       string   `json:"keyField,omitempty"`
	Excludes       []Filter `json:"excludes,omitempty"`
}

type ElasticsearchAuthSpec struct {
	Password      string `json:"password,omitempty"`
	Strategy      string `json:"strategy,omitempty"`
	User          string `json:"user,omitempty"`
	AwsAccessKey  string `json:"awsAccessKey,omitempty"`
	AwsSecretKey  string `json:"awsSecretAccessKey,omitempty"`
	AwsAssumeRole string `json:"awsAssumeRole,omitempty"`
	AwsRegion     string `json:"awsRegion,omitempty"`
}

type CommonTLSClientCert struct {
	CertFile string `json:"crtFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
	KeyPass  string `json:"keyPass,omitempty"`
}

type CommonTLSSpec struct {
	CommonTLSClientCert `json:"clientCrt,omitempty"`
	CAFile              string `json:"caFile,omitempty"`
	VerifyHostname      *bool  `json:"verifyHostname,omitempty"`
	VerifyCertificate   *bool  `json:"verifyCertificate,omitempty"`
}

type EncodingCodec = string

const (
	EncodingCodecText   EncodingCodec = "Text"
	EncodingCodecCEF    EncodingCodec = "CEF"
	EncodingCodecJSON   EncodingCodec = "JSON"
	EncodingCodecSyslog EncodingCodec = "Syslog"
)

type CommonEncoding struct {
	Codec EncodingCodec `json:"codec"`
}

type LokiSpec struct {
	// TenantID is used only for GrafanaCloud. When running Loki locally, a tenant ID is not required.
	TenantID string `json:"tenantID,omitempty"`

	Endpoint string `json:"endpoint,omitempty"`

	Auth LokiAuthSpec `json:"auth,omitempty"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type KafkaSpec struct {
	BootstrapServers []string `json:"bootstrapServers,omitempty"`

	Topic    string        `json:"topic,omitempty"`
	KeyField string        `json:"keyField,omitempty"`
	TLS      CommonTLSSpec `json:"tls,omitempty"`

	SASL KafkaSASL `json:"sasl,omitempty"`

	Encoding CommonEncoding `json:"encoding,omitempty"`
}

type KafkaSASLMechanism string

const (
	KafkaSASLMechanismPLAIN  KafkaSASLMechanism = "PLAIN"
	KafkaSASLMechanismSHA256 KafkaSASLMechanism = "SCRAM-SHA-256"
	KafkaSASLMechanismSHA512 KafkaSASLMechanism = "SCRAM-SHA-512"
)

type KafkaSASL struct {
	Username  string             `json:"username"`
	Password  string             `json:"password"`
	Mechanism KafkaSASLMechanism `json:"mechanism"`
}

type ElasticsearchSpec struct {
	Endpoint string `json:"endpoint,omitempty"`

	Index    string `json:"index,omitempty"`
	Pipeline string `json:"pipeline,omitempty"`
	Type     string `json:"type,omitempty"`

	Auth              ElasticsearchAuthSpec `json:"auth,omitempty"`
	DataStreamEnabled bool                  `json:"dataStreamEnabled"`
	DocType           string                `json:"docType"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type LogstashSpec struct {
	Endpoint string `json:"endpoint,omitempty"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type VectorSpec struct {
	Endpoint string `json:"endpoint,omitempty"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type SplunkSpec struct {
	Endpoint string `json:"endpoint,omitempty"`

	Token string `json:"token,omitempty"`

	Index string `json:"index,omitempty"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type SocketSpec struct {
	Address string `json:"address,omitempty"`

	Mode SocketMode `json:"mode,omitempty"`

	Encoding CommonEncoding `json:"encoding,omitempty"`

	TCP SocketTCPSpec `json:"tcp,omitempty"`
}

type SocketMode = string

const (
	SocketModeTCP SocketMode = "TCP"
	SocketModeUDP SocketMode = "UDP"
)

type SocketTCPSpec struct {
	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type Buffer struct {
	// The type of buffer to use.
	Type BufferType `json:"type,omitempty"`

	// Relevant when: type = "disk"
	Disk BufferDisk `json:"disk,omitempty"`

	// Relevant when: type = "memory"
	Memory BufferMemory `json:"memory,omitempty"`

	// Event handling behavior when a buffer is full.
	WhenFull BufferWhenFull `json:"whenFull,omitempty"`
}

type BufferType = string

const (
	// BufferTypeDisk specifies that events are buffered on disk.
	// This is less performant, but more durable. Data that has been synchronized to disk will not be lost if Vector is restarted forcefully or crashes.
	// Data is synchronized to disk every 500ms.
	BufferTypeDisk BufferType = "Disk"

	// BufferTypeMemory specifies that events are buffered in memory.
	// This is more performant, but less durable. Data will be lost if Vector is restarted forcefully or crashes.
	BufferTypeMemory BufferType = "Memory"
)

type BufferWhenFull = string

const (
	// BufferWhenFullDropNewest makes vector dropping the event instead of waiting for free space in buffer.
	// The event will be intentionally dropped. This mode is typically used when performance is the highest priority,
	// and it is preferable to temporarily lose events rather than cause a slowdown in the acceptance/consumption of events.
	BufferWhenFullDropNewest BufferWhenFull = "DropNewest"

	// BufferWhenFullBlock makes vector waiting for free space in the buffer.
	// This applies backpressure up the topology, signalling that sources should slow down the acceptance/consumption of events. This means that while no data is lost, data will pile up at the edge.
	BufferWhenFullBlock BufferWhenFull = "Block"
)

type BufferDisk struct {
	// 	The maximum size of the buffer on disk.
	// Must be at least ~256 megabytes (268435488 bytes).
	MaxSize resource.Quantity `json:"maxSize,omitempty"`
}

type BufferMemory struct {
	// The maximum number of events allowed in the buffer.
	MaxEvents uint32 `json:"maxEvents,omitempty"`
}
