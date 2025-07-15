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
	"sort"
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
		if spec.TCP.TLS.CAFile != "" || spec.TCP.TLS.CertFile != "" {
			tls.Enabled = true
		}

		result.TLS = tls
	}

	encoding := Encoding{TimestampFormat: "rfc3339"}

	switch spec.Encoding.Codec {
	case v1alpha1.EncodingCodecText:
		encoding.Codec = "text"
	case v1alpha1.EncodingCodecSyslog:
		encoding.Codec = "text"
		// the main encoding is done by the vrl rule
	case v1alpha1.EncodingCodecCEF:
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
		extensions := map[string]string{
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
		}

		keys := make([]string, 0, len(cspec.ExtraLabels))
		for key := range cspec.ExtraLabels {
			keys = append(keys, key)
		}

		sort.Strings(keys)
		specialKeys := map[string]struct{}{
			"cef.name":     {},
			"cef.severity": {},
		}

		for _, k := range keys {
			normalized := normalizeKey(k)
			if _, isSpecial := specialKeys[normalized]; isSpecial {
				continue
			}
			extensions[normalized] = k
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
			Extensions:         extensions,
		}

	case v1alpha1.EncodingCodecGELF:
		encoding.Codec = "gelf"
	default:
		encoding.Codec = "json"
	}

	result.Encoding = encoding
	return result
}
