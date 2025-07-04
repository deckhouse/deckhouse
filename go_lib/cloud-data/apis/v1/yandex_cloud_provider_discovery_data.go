// Copyright 2024 Flant JSC
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
	"net/netip"
	"time"
)

type DiscoveryData struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`

	Zones     []string             `json:"zones,omitempty"     yaml:"zones,omitempty"`
	Images    []DiscoveryDataImage `json:"images,omitempty"    yaml:"images,omitempty"`
	DiskTypes []string             `json:"diskTypes,omitempty" yaml:"diskTypes,omitempty"`

	Networks          []DiscoveryDataNetwork         `json:"networks,omitempty"`
	Subnets           []DiscoveryDataSubnet          `json:"subnets,omitempty"`
	Platforms         []string                       `json:"platforms,omitempty"`
	ExternalAddresses []DiscoveryDataExternalAddress `json:"externalAddresses,omitempty"`
}
type DiscoveryDataImage struct {
	ImageID     string    `json:"imageId,omitempty"   yaml:"imageId,omitempty"`
	Name        string    `json:"name,omitempty"      yaml:"name,omitempty"`
	Family      string    `json:"family,omitempty"    yaml:"family,omitempty"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
}

// TODO: omit empty?
type DiscoveryDataNetwork struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type DiscoveryDataSubnet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Id of network subnet belongs to.
	NetworkId string    `json:"networkId"`
	V4cidr    string    `json:"v4cidr"`
	CreatedAt time.Time `json:"createdAt"`
	Zone      string    `json:"zone"`
}

type DiscoveryDataExternalAddress struct {
	ID   string `json:"id"`
	Zone string `json:"zone"`
	// Ipv4 external address.
	IP   netip.Addr `json:"ip"`
	Used bool       `json:"used"`
}
