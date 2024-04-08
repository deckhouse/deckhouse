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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

var validMustacheTemplate = regexp.MustCompile(`^\{\{\ ([a-zA-Z0-9][a-zA-Z0-9\[\]_\\\-\.]+)\ \}\}$`)

type Loki struct {
	CommonSettings

	Encoding Encoding `json:"encoding,omitempty"`

	TenantID string `json:"tenant_id,omitempty"`

	Endpoint string `json:"endpoint"`

	Auth LokiAuth `json:"auth,omitempty"`

	TLS CommonTLS `json:"tls"`

	Labels map[string]string `json:"labels,omitempty"`

	RemoveLabelFields bool `json:"remove_label_fields"`

	OutOfOrderAction string `json:"out_of_order_action"`
}

type LokiAuth struct {
	Password string `json:"password,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Token    string `json:"token,omitempty"`
	User     string `json:"user,omitempty"`
}

func NewLoki(name string, cspec v1alpha1.ClusterLogDestinationSpec) *Loki {
	spec := cspec.Loki

	// default labels
	//
	// Asterisk is required here to expand all pod labels
	// See https://github.com/vectordotdev/vector/pull/12041
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
		// File labels
		// TODO(nabokihms): think about removing this label and always use the `node` labels.
		//   If we do this right now, it will break already working setups.
		"host": "{{ host }}",
		// "file": "{{ file }}", The file label is excluded due to potential cardinality bomb
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

	return &Loki{
		CommonSettings: CommonSettings{
			Name:   ComposeName(name),
			Type:   "loki",
			Inputs: set.New(),
			Buffer: buildVectorBuffer(cspec.Buffer),
		},
		Auth: LokiAuth{
			User:     spec.Auth.User,
			Token:    spec.Auth.Token,
			Strategy: strings.ToLower(spec.Auth.Strategy),
			Password: decodeB64(spec.Auth.Password),
		},
		TLS:      tls,
		Labels:   labels,
		Endpoint: spec.Endpoint,
		TenantID: spec.TenantID,
		Encoding: Encoding{
			Codec:           "text",
			TimestampFormat: "rfc3339",
			OnlyFields:      []string{"message"},
		},
		RemoveLabelFields: true,
		OutOfOrderAction:  "rewrite_timestamp",
	}
}
