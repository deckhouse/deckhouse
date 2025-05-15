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
	"fmt"
	"sort"

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

	IndexedFields []string `json:"indexed_fields,omitempty"`

	TLS CommonTLS `json:"tls"`

	Labels map[string]string `json:"labels,omitempty"`
}

func NewSplunk(name string, cspec v1alpha1.ClusterLogDestinationSpec) *Splunk {
	spec := cspec.Splunk

	labels := map[string]string{
		// Kubernetes logs labels
		"namespace":    "{{ namespace }}",
		"container":    "{{ container }}",
		"image":        "{{ image }}",
		"pod":          "{{ pod }}",
		"node":         "{{ node }}",
		"pod_ip":       "{{ pod_ip }}",
		"stream":       "{{ stream }}",
		"pod_labels_*": "{{ pod_labels }}",
		"node_group":   "{{ node_group }}",
		"pod_owner":    "{{ pod_owner }}",
		"host":         "{{ host }}",
	}

	var dataField string
	keys := make([]string, 0, len(cspec.ExtraLabels))
	for key := range cspec.ExtraLabels {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, k := range keys {
		if validMustacheTemplate.MatchString(cspec.ExtraLabels[k]) {
			dataField = validMustacheTemplate.FindStringSubmatch(cspec.ExtraLabels[k])[1]
			labels[k] = fmt.Sprintf("{{ parsed_data.%s }}", dataField)
		} else {
			labels[k] = cspec.ExtraLabels[k]
		}
	}

	tls := CommonTLS{
		CAFile:            decodeB64(spec.TLS.CAFile),
		CertFile:          decodeB64(spec.TLS.CertFile),
		KeyFile:           decodeB64(spec.TLS.KeyFile),
		KeyPass:           decodeB64(spec.TLS.KeyPass),
		VerifyCertificate: true,
		VerifyHostname:    true,
	}
	if spec.TLS.VerifyCertificate != nil {
		tls.VerifyCertificate = *spec.TLS.VerifyCertificate
	}
	if spec.TLS.VerifyHostname != nil {
		tls.VerifyHostname = *spec.TLS.VerifyHostname
	}

	indexedFields := []string{
		"datetime",
		"namespace",
		"container",
		"image",
		"pod",
		"node",
		"pod_ip",
		"stream",
		"pod_owner",
		"host",
		// "pod_labels", Splunk does not support objects with dynamic keys for indexes, consider using extraLabels
	}

	// Send extra labels as indexed fields
	for k := range cspec.ExtraLabels {
		indexedFields = append(indexedFields, k)
	}

	return &Splunk{
		CommonSettings: CommonSettings{
			Name:   ComposeName(name),
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
		IndexedFields: indexedFields,
		Endpoint:      spec.Endpoint,
		DefaultToken:  spec.Token,
		Labels:        labels,
		Compression:   "gzip",
	}
}
