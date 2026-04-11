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

func NewSocket(sinkName string, cspec v1alpha1.ClusterLogDestinationSpec, cefExtensions map[string]string) *Socket {
	spec := cspec.Socket

	result := &Socket{
		CommonSettings: CommonSettings{
			Name:   sinkName,
			Type:   "socket",
			Inputs: set.New(),
			Buffer: buildVectorBuffer(cspec.Buffer),
		},
		Address: spec.Address,
		Mode:    strings.ToLower(spec.Mode),
	}

	if spec.Mode == v1alpha1.SocketModeTCP {
		result.TLS = commonTLSFromSpecWithClientEnabled(spec.TCP.TLS)
	}

	encoding := Encoding{TimestampFormat: "rfc3339"}

	switch spec.Encoding.Codec {
	case v1alpha1.EncodingCodecText:
		encoding.Codec = "text"
	case v1alpha1.EncodingCodecSyslog:
		encoding.Codec = "text"
		// the main encoding is done by the vrl rule
	case v1alpha1.EncodingCodecCEF:
		encoding.Codec = "cef"
		encoding.CEF = cefEncodingFromCRD(spec.Encoding.CEF, cefExtensions)

	case v1alpha1.EncodingCodecGELF:
		encoding.Codec = "gelf"
	default:
		encoding.Codec = "json"
	}

	result.Encoding = encoding
	return result
}
