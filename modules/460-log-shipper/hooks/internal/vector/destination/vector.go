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

type Vector struct {
	CommonSettings

	Version string `json:"version,omitempty"`

	Address string `json:"address"`

	TLS CommonTLS `json:"tls"`

	Keepalive VectorKeepalive `json:"keepalive,omitempty"`
}

type VectorKeepalive struct {
	TimeSecs int `json:"time_secs"`
}

func NewVector(name string, cspec v1alpha1.ClusterLogDestinationSpec) *Vector {
	spec := cspec.Vector

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

	return &Vector{
		CommonSettings: CommonSettings{
			Name:   ComposeName(name),
			Type:   "vector",
			Inputs: set.New(),
			Buffer: buildVectorBuffer(cspec.Buffer),
		},
		TLS:     tls,
		Version: "2",
		Address: spec.Endpoint,
		// TODO(nabokihms): Only available for vector the first version sink, consider different load balancing solution
		//
		// Keepalive: VectorKeepalive{
		//	TimeSecs: 7200,
		// },
	}
}
