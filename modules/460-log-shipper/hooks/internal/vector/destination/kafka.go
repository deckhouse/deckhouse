// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package destination

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
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

	Labels map[string]string `json:"labels,omitempty"`
}

type KafkaSASL struct {
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	Mechanism string `json:"mechanism,omitempty"`

	Enabled bool `json:"enabled,omitempty"`
}

func NewKafka(name string, cspec v1alpha1.ClusterLogDestinationSpec) *Kafka {
	spec := cspec.Kafka

	var labels map[string]string

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
	if len(tls.CAFile) > 0 || len(tls.CertFile) > 0 {
		tls.Enabled = true
	}

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
	if spec.Encoding.Codec == v1alpha1.EncodingCodecCEF {
		deviceVendor := "Deckhouse"
		if spec.Encoding.CEF.DeviceVendor != "" {
			deviceVendor = spec.Encoding.CEF.DeviceVendor
		}

		deviceProduct := "log-shipper-agent"
		if spec.Encoding.CEF.DeviceProduct != "" {
			deviceProduct = spec.Encoding.CEF.DeviceProduct
		}

		deviceVersion := "1"
		if spec.Encoding.CEF.DeviceVersion != "" {
			deviceVersion = spec.Encoding.CEF.DeviceVersion
		}
		encoding.Codec = "cef"
		encoding.CEF = CEFEncoding{
			Version:            "V1",
			DeviceVendor:       deviceVendor,
			DeviceProduct:      deviceProduct,
			DeviceVersion:      deviceVersion,
			DeviceEventClassID: "Log event",
			Name:               "cef.name",
			Severity:           "cef.severity",
			Extensions: map[string]string{
				"message":   "message",
				"timestamp": "timestamp",
				"node":      "node",
				"host":      "host",
				"pod":       "pod",
				"podip":     "pod_ip",
				"namespace": "namespace",
				"image":     "image",
				"container": "container",
				"podowner":  "pod_owner",
			},
		}
	}

	return &Kafka{
		CommonSettings: CommonSettings{
			Name:   ComposeName(name),
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
		Labels:           labels,
		BootstrapServers: strings.Join(spec.BootstrapServers, ","),
	}
}
