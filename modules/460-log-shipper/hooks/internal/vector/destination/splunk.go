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
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

type Splunk struct {
	CommonSettings

	Encoding Encoding `json:"encoding,omitempty"`

	Compression string `json:"compression,omitempty"`

	DefaultToken string `json:"default_token,omitempty"`

	Endpoint string `json:"endpoint,omitempty"`

	Index string `json:"index,omitempty"`

	IndexedFields IndexedFieldsMap `json:"indexed_fields,omitempty"`

	TLS CommonTLS `json:"tls"`
}

func NewSplunk(sinkName string, cspec v1alpha1.ClusterLogDestinationSpec, indexedFields map[string]string) *Splunk {
	spec := cspec.Splunk

	tls := commonTLSFromSpec(spec.TLS)

	return &Splunk{
		CommonSettings: CommonSettings{
			Name:   sinkName,
			Type:   "splunk_hec_logs",
			Inputs: set.New(),
			Buffer: buildVectorBuffer(cspec.Buffer),
		},
		TLS:   tls,
		Index: spec.Index,
		Encoding: Encoding{
			OnlyFields:      []string{"message"}, // Do not encode fields used in indexes
			Codec:           "text",
			TimestampFormat: "rfc3339",
		},
		IndexedFields: IndexedFieldsMap(indexedFields),
		Endpoint:      spec.Endpoint,
		DefaultToken:  spec.Token,
		Compression:   "gzip",
	}
}
