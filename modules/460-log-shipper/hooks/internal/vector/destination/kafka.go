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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha2"
)

type Kafka struct {
	CommonSettings

	BootstrapServers string `json:"bootstrap_servers,omitempty"`

	Encoding Encoding `json:"encoding,omitempty"`

	Topic       string `json:"topic"`
	KeyField    string `json:"key_field,omitempty"`
	Compression string `json:"compression,omitempty"`

	TLS CommonTLS `json:"tls"`

	SASL KafkaSASL `json:"sasl,omitempty"`
}

type KafkaSASL struct {
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	Mechanism string `json:"mechanism,omitempty"`

	Enabled bool `json:"enabled,omitempty"`
}

func NewKafka(sinkName string, cspec v1alpha2.ClusterLogDestinationSpec, cefExtensions map[string]string) *Kafka {
	spec := cspec.Kafka

	tls := commonTLSFromSpecWithClientEnabled(spec.TLS)

	sasl := KafkaSASL{
		Enabled:   false,
		Username:  spec.SASL.Username,
		Password:  spec.SASL.Password,
		Mechanism: string(spec.SASL.Mechanism),
	}
	if sasl.Mechanism != "" && sasl.Username != "" && sasl.Password != "" {
		sasl.Enabled = true
	}

	encoding := Encoding{
		Codec:           "json",
		TimestampFormat: "rfc3339",
	}
	// Get CEF extensions based on source type (uses K8sLabels and FilesLabels)
	if spec.Encoding.Codec == v1alpha2.EncodingCodecCEF {
		encoding.Codec = "cef"
		encoding.CEF = cefEncodingFromCRD(spec.Encoding.CEF, cefExtensions)
	}

	return &Kafka{
		CommonSettings: CommonSettings{
			Name:   sinkName,
			Type:   "kafka",
			Inputs: set.New(),
			Buffer: buildVectorBuffer(cspec.Buffer),
		},
		TLS:              tls,
		Topic:            spec.Topic,
		Encoding:         encoding,
		SASL:             sasl,
		KeyField:         spec.KeyField,
		Compression:      "gzip",
		BootstrapServers: strings.Join(spec.BootstrapServers, ","),
	}
}
