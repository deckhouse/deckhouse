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

package v1alpha2

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

type ClusterLogDestination struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterLogDestinationSpec   `json:"spec"`
	Status ClusterLogDestinationStatus `json:"status,omitempty"`
}

type ClusterLogDestinationSpec struct {
	Type string `json:"type,omitempty"`

	Loki          LokiSpec          `json:"loki"`
	Elasticsearch ElasticsearchSpec `json:"elasticsearch"`
	Logstash      LogstashSpec      `json:"logstash"`
	Kafka         KafkaSpec         `json:"kafka"`
	Splunk        SplunkSpec        `json:"splunk"`
	Socket        SocketSpec        `json:"socket"`
	Vector        VectorSpec        `json:"vector"`

	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	RateLimit RateLimitSpec `json:"rateLimit,omitempty"`

	Buffer          *Buffer              `json:"buffer,omitempty"`
	Transformations []TransformationSpec `json:"transformations,omitempty"`
}

type ClusterLogDestinationStatus struct {
}

type RateLimitSpec struct {
	LinesPerMinute *int32            `json:"linesPerMinute,omitempty"`
	KeyField       string            `json:"keyField,omitempty"`
	Excludes       []v1alpha1.Filter `json:"excludes,omitempty"`
}

type LokiAuthSpec struct {
	Password string `json:"password,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Token    string `json:"token,omitempty"`
	User     string `json:"user,omitempty"`
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
	SecretRef           *SecretRef `json:"secretRef,omitempty"`
	CAFile              string     `json:"caFile,omitempty"`
	VerifyHostname      *bool      `json:"verifyHostname,omitempty"`
	VerifyCertificate   *bool      `json:"verifyCertificate,omitempty"`
}

type SecretRef struct {
	Name string `json:"name,omitempty"`
}

type EncodingCodec = string

const (
	EncodingCodecText   EncodingCodec = "Text"
	EncodingCodecCEF    EncodingCodec = "CEF"
	EncodingCodecJSON   EncodingCodec = "JSON"
	EncodingCodecSyslog EncodingCodec = "Syslog"
	EncodingCodecGELF   EncodingCodec = "GELF"
)

type CEFEncoding struct {
	DeviceVendor  string `json:"deviceVendor,omitempty"`
	DeviceProduct string `json:"deviceProduct,omitempty"`
	DeviceVersion string `json:"deviceVersion,omitempty"`
}

type CommonEncoding struct {
	Codec EncodingCodec `json:"codec"`
	CEF   CEFEncoding   `json:"cef,omitempty"`
}

type LokiSpec struct {
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
	Type BufferType `json:"type,omitempty"`

	Disk BufferDisk `json:"disk,omitempty"`

	Memory BufferMemory `json:"memory,omitempty"`

	WhenFull BufferWhenFull `json:"whenFull,omitempty"`
}

type BufferType = string

const (
	BufferTypeDisk BufferType = "Disk"

	BufferTypeMemory BufferType = "Memory"
)

type BufferWhenFull = string

const (
	BufferWhenFullDropNewest BufferWhenFull = "DropNewest"

	BufferWhenFullBlock BufferWhenFull = "Block"
)

type BufferDisk struct {
	MaxSize resource.Quantity `json:"maxSize,omitempty"`
}

type BufferMemory struct {
	MaxEvents uint32 `json:"maxEvents,omitempty"`
}
