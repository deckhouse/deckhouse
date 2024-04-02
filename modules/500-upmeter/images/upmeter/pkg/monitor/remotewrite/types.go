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

package remotewrite

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config for sending metrics via Prometheus Remote Write 1.0 Protocol. Used for metrics export. Refer to cortex.Config.
type Config struct {
	Endpoint    string            `json:"url"`
	BasicAuth   map[string]string `json:"basicAuth"`
	BearerToken string            `json:"bearerToken"`
	TLSConfig   TLSConfig         `json:"tlsConfig"`
}

// TLSConfig is the spec in the RemoteWrite CRD
type TLSConfig struct {
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	CA                 string `json:"ca"`
}

// Spec is the spec in the RemoteWrite CRD
type Spec struct {
	Config           Config            `json:"config"`
	AdditionalLabels map[string]string `json:"additionalLabels"`
	IntervalSeconds  int64             `json:"intervalSeconds"`
}

// RemoteWrite is the Schema for the upmeterremotewrites.deckhouse.io
type RemoteWrite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Spec `json:"spec,omitempty"`
}

// RemoteWriteList contains a list of RemoteWrite objects
type RemoteWriteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RemoteWrite `json:"items"`
}
