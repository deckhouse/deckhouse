// Copyright 2023 Flant JSC
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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "volumetypescatalogs",
}

type VolumeTypesCatalog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	VolumeTypes []VolumeType `json:"volumeTypes"`
}

type VolumeType struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Parameters map[string]any `json:"parameters"`
}

var GVRDiscoveryData = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "discoverydatasets",
}

type DiscoveryDataset struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DiscoveryData
}

type DiscoveryData struct {
	MainNetwork      string   `json:"mainNetwork"`
	Images           []Image  `json:"images"`
	DefaultImageName string   `json:"defaultImageName"`
	Zones            []string `json:"zones"`
}

type Image struct {
	Name string `json:"name"`
}
