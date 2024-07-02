/*
Copyright 2024 Flant JSC

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

type Socket struct {
	CommonSettings

	Encoding Encoding `json:"encoding,omitempty"`

	Mode string `json:"mode,omitempty"`

	Address string `json:"address,omitempty"`

	TLS CommonTLS `json:"tls,omitempty"`
}

func NewSocket(name string, cspec v1alpha1.ClusterLogDestinationSpec) *Socket {
	spec := cspec.Socket

	result := &Socket{
		CommonSettings: CommonSettings{
			Name:   ComposeName(name),
			Type:   "socket",
			Inputs: set.New(),
			Buffer: buildVectorBuffer(cspec.Buffer),
		},
		Address: spec.Address,
		Mode:    strings.ToLower(spec.Mode),
	}

	if spec.Mode == v1alpha1.SocketModeTCP {
		tls := CommonTLS{
			CAFile:            decodeB64(spec.TCP.TLS.CAFile),
			CertFile:          decodeB64(spec.TCP.TLS.CertFile),
			KeyFile:           decodeB64(spec.TCP.TLS.KeyFile),
			KeyPass:           decodeB64(spec.TCP.TLS.KeyPass),
			VerifyCertificate: true,
			VerifyHostname:    true,
		}
		if spec.TCP.TLS.VerifyCertificate != nil {
			tls.VerifyCertificate = *spec.TCP.TLS.VerifyCertificate
		}
		if spec.TCP.TLS.VerifyHostname != nil {
			tls.VerifyHostname = *spec.TCP.TLS.VerifyHostname
		}

		result.TLS = tls
	}

	encoding := Encoding{
		Codec:           "json",
		TimestampFormat: "rfc3339",
	}
	if spec.Encoding.Codec == v1alpha1.EncodingCodecText {
		encoding.Codec = "text"
	}
	if spec.Encoding.Codec == v1alpha1.EncodingCodecSyslog {
		encoding.Codec = "text"
		// the main encoding is done by the vrl rule
	}
	if spec.Encoding.Codec == v1alpha1.EncodingCodecCEF {
		encoding.Codec = "cef"
		encoding.CEF = CEFEncoding{
			Version:            "V1",
			DeviceVendor:       "Deckhouse",
			DeviceProduct:      "log-shipper-agent",
			DeviceVersion:      "1",
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
	result.Encoding = encoding

	return result
}
