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

package destination

import (
	"strings"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

type Elasticsearch struct {
	CommonSettings

	Endpoint string `json:"endpoint"`

	Encoding ElasticsearchEncoding `json:"encoding,omitempty"`

	Batch ElasticsearchBatch `json:"batch,omitempty"`

	Auth ElasticsearchAuth `json:"auth,omitempty"`

	TLS CommonTLS `json:"tls,omitempty"`

	AWS ElasticsearchRegion `json:"aws,omitempty"`

	Compression string `json:"compression,omitempty"`

	Bulk ElasticsearchBulk `json:"bulk,omitempty"`

	Pipeline string `json:"pipeline,omitempty"`

	Mode string `json:"mode,omitempty"`

	DocType string `json:"doc_type,omitempty"`
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

type ElasticsearchBatch struct {
	MaxSize     uint32 `json:"max_bytes,omitempty"`
	TimeoutSecs uint32 `json:"timeout_secs,omitempty"`
}

type ElasticsearchRegion struct {
	Region string `json:"region,omitempty"`
}

type ElasticsearchBulk struct {
	Action string `json:"action,omitempty"`
	Index  string `json:"index,omitempty"`
}

func NewElasticsearch(name string, cspec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	spec := cspec.Elasticsearch

	// Disable buffer. It is buggy. Vector developers know about problems with buffer.
	// More info about buffer rewriting here - https://github.com/vectordotdev/vector/issues/9476
	// common.Buffer = buffer{
	//	Size: 100 * 1024 * 1024, // 100MiB in bytes for vector persistent queue
	//	Type: "disk",
	// }

	bulkAction := "index"
	mode := "bulk"

	if spec.DataStreamEnabled {
		bulkAction = "create"
		mode = "data_stream"
	}

	return &Elasticsearch{
		CommonSettings: CommonSettings{
			Name: "d8_cluster_sink_" + name,
			Type: "elasticsearch",
		},
		Auth: ElasticsearchAuth{
			AwsAccessKey:  decodeB64(spec.Auth.AwsAccessKey),
			AwsSecretKey:  decodeB64(spec.Auth.AwsSecretKey),
			AwsAssumeRole: spec.Auth.AwsAssumeRole,
			User:          spec.Auth.User,
			Password:      decodeB64(spec.Auth.Password),
			Strategy:      strings.ToLower(spec.Auth.Strategy),
		},
		Encoding: ElasticsearchEncoding{
			TimestampFormat: "rfc3339",
		},
		TLS: CommonTLS{
			CAFile:         decodeB64(spec.TLS.CAFile),
			CertFile:       decodeB64(spec.TLS.CertFile),
			KeyFile:        decodeB64(spec.TLS.KeyFile),
			KeyPass:        decodeB64(spec.TLS.KeyPass),
			VerifyHostname: spec.TLS.VerifyHostname,
		},
		AWS: ElasticsearchRegion{
			Region: spec.Auth.AwsRegion,
		},
		Batch: ElasticsearchBatch{
			MaxSize:     10 * 1024 * 1024, // 10MiB in bytes for elasticsearch bulk api
			TimeoutSecs: 1,
		},
		Bulk: ElasticsearchBulk{
			Action: bulkAction,
			Index:  spec.Index,
		},
		Endpoint:    spec.Endpoint,
		Pipeline:    spec.Pipeline,
		Compression: "gzip",
		DocType:     spec.DocType,
		Mode:        mode,
	}
}
