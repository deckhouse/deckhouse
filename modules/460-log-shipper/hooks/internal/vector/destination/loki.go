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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

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

func NewLoki(sinkName string, cspec v1alpha1.ClusterLogDestinationSpec, labels map[string]string) *Loki {
	spec := cspec.Loki

	tls := commonTLSFromSpec(spec.TLS)

	return &Loki{
		CommonSettings: CommonSettings{
			Name:   sinkName,
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
