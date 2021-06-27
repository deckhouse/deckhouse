/*
Copyright 2021 Flant CJSC

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// refer to cortex.Config
type RemoteWriteConfig struct {
	Endpoint    string            `json:"url"`
	BasicAuth   map[string]string `json:"basicAuth"`
	BearerToken string            `json:"bearerToken"`
}

type RemoteWriteSpec struct {
	Config           RemoteWriteConfig `json:"config"`
	AdditionalLabels map[string]string `json:"additionalLabels"`
	IntervalSeconds  int64             `json:"intervalSeconds"`
}

// RemoteWrite is the Schema for the remote_write settings
type RemoteWrite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RemoteWriteSpec `json:"spec,omitempty"`
}

// DowntimeList contains a list of DowntimeIncident
type RemoteWriteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RemoteWrite `json:"items"`
}
