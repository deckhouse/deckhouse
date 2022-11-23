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

	// Vector spec for the Vector endpoint
	Vector VectorSpec `json:"vector"`

	// Add extra labels for sources
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	// Add rateLimit for sink
	RateLimit RateLimitSpec `json:"rateLimit,omitempty"`
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
	LinesPerMinute *int32 `json:"linesPerMinute,omitempty"`
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

type LokiSpec struct {
	Endpoint string `json:"endpoint,omitempty"`

	Auth LokiAuthSpec `json:"auth,omitempty"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
}

type KafkaSpec struct {
	BootstrapServers []string `json:"bootstrapServers,omitempty"`

	Topic string `json:"topic,omitempty"`

	TLS CommonTLSSpec `json:"tls,omitempty"`
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
