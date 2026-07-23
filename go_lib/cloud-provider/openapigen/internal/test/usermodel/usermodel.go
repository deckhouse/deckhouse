/*
Copyright 2026 Flant JSC

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

package usermodel

import (
	"openapigen/internal/test/testmodel"
)

type id string

// The user definition
// +deckhouse:DisableAdditionalProperties=true
type User struct {
	// The user's ID
	//
	// UUID required
	// +deckhouse:ru:description=Идентификатор пользователя
	// +deckhouse:ru:description=
	// +deckhouse:ru:description=требуется UUID
	// +kubebuilder:default=deadbeefcafe
	// +kubebuilder:title=User ID
	// +kubebuilder:example=1234567890
	// +kubebuilder:validation:Pattern=^[0-9]+$
	// +kubebuilder:validation:Format=int64
	// +required
	ID id `json:"id,omitempty"`
	// +required
	Name string `json:"name,omitempty"`
	// The user's email
	// +required
	Email string `json:"email,omitempty"`
	// The user's phone
	Phone string `json:"phone,omitempty"`
	// The user's geo definition
	// +required
	Geo Geo `json:"geo,omitempty"`
	// User owner definition
	// +required
	Owner *Owner `json:"owner,omitempty"`
	// Good reference
	// +required
	Good *testmodel.GoodRef `json:"good,omitempty"`
}

// The geo definition
// +kubebuilder:title=Geo definition
type Geo struct {
	// The geo object identificator
	// +kubebuilder:validation:Pattern=.*
	// +required
	ID id `json:"id,omitempty"`
	// The geoposition latitude
	// +kubebuilder:validation:MinLength=6
	// +kubebuilder:validation:MaxLength=6
	// +required
	Lat int `json:"lat,omitempty"`
	// The geoposition longitude
	// +kubebuilder:validation:MinLength=6
	// +kubebuilder:validation:MaxLength=6
	// +required
	Lon string `json:"lon,omitempty"`
	// +optional
	Address string `json:"address,omitempty"`
	// +optional
	City string `json:"city,omitempty"`
	// +optional
	State string `json:"state,omitempty"`
	// +optional
	Zip string `json:"zip,omitempty"`
	// +optional
	Country string `json:"country,omitempty"`
	// +deckhouse:ru:description=Объявление владельца геопозиции
	// +optional
	OwnerRef *Owner `json:"ownerReference,omitempty"`
}

// +deckhouse:ru:description=Объявление владельца
// +kubebuilder:title=Owner definition
// Owner definition
type Owner struct {
	// The owner's ID
	ID id `json:"id"`
	// The owner's name
	Name string `json:"name"`
}
