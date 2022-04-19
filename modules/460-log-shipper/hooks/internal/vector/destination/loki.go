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
	"regexp"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

var validMustacheTemplate = regexp.MustCompile(`^\{\{\ ([a-zA-Z0-9][a-zA-Z0-9\[\]_\\\-\.]+)\ \}\}$`)

type Loki struct {
	CommonSettings

	Encoding LokiEncoding `json:"encoding,omitempty"`

	Endpoint string `json:"endpoint"`

	Auth LokiAuth `json:"auth,omitempty"`

	TLS CommonTLS `json:"tls,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	RemoveLabelFields bool `json:"remove_label_fields"`

	OutOfOrderAction string `json:"out_of_order_action"`
}

type LokiEncoding struct {
	Codec           string   `json:"codec,omitempty"`
	OnlyFields      []string `json:"only_fields,omitempty"`
	TimestampFormat string   `json:"timestamp_format,omitempty"`
}

type LokiAuth struct {
	Password string `json:"password,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Token    string `json:"token,omitempty"`
	User     string `json:"user,omitempty"`
}

func NewLoki(name string, cspec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	spec := cspec.Loki

	// Disable buffer. It is buggy. Vector developers know about problems with buffer.
	// More info about buffer rewriting here - https://github.com/vectordotdev/vector/issues/9476
	// common.Buffer = buffer{
	//	Size: 100 * 1024 * 1024, // 100MiB in bytes for vector persistent queue
	//	Type: "disk",
	// }

	// default labels
	//
	// Asterisk is required here to expand all pod labels
	// See https://github.com/vectordotdev/vector/pull/12041
	labels := map[string]string{
		"namespace":    "{{ namespace }}",
		"container":    "{{ container }}",
		"image":        "{{ image }}",
		"pod":          "{{ pod }}",
		"node":         "{{ node }}",
		"pod_ip":       "{{ pod_ip }}",
		"stream":       "{{ stream }}",
		"pod_labels_*": "{{ pod_labels }}",
		"pod_owner":    "{{ pod_owner }}",
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

	return &Loki{
		CommonSettings: CommonSettings{
			Name: "d8_cluster_sink_" + name,
			Type: "loki",
		},
		Auth: LokiAuth{
			User:     spec.Auth.User,
			Token:    spec.Auth.Token,
			Strategy: strings.ToLower(spec.Auth.Strategy),
			Password: decodeB64(spec.Auth.Password),
		},
		TLS: CommonTLS{
			CAFile:         decodeB64(spec.TLS.CAFile),
			CertFile:       decodeB64(spec.TLS.CertFile),
			KeyFile:        decodeB64(spec.TLS.KeyFile),
			KeyPass:        decodeB64(spec.TLS.KeyPass),
			VerifyHostname: spec.TLS.VerifyHostname,
		},
		Labels:   labels,
		Endpoint: spec.Endpoint,
		Encoding: LokiEncoding{
			Codec:           "text",
			TimestampFormat: "rfc3339",
			OnlyFields:      []string{"message"},
		},
		RemoveLabelFields: true,
		OutOfOrderAction:  "rewrite_timestamp",
	}
}
