package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// refer to cortex.Config
type RemoteWriteConfig struct {
	Endpoint  string            `json:"url"`
	BasicAuth map[string]string `json:"basicAuth"`
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
