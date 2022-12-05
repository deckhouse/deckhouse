/*
Copyright 2022 Flant JSC

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

package dynamic_probe

// names contains the data for dynamic probes. Initially it was created for the hook tests,
// but it also appeared to be handy for the hook itself.
type names struct {
	IngressControllerNames       []string `json:"ingressControllerNames"`
	CloudEphemeralNodeGroupNames []string `json:"cloudEphemeralNodeGroupNames"`
	Zones                        []string `json:"zones"`
	ZonePrefix                   string   `json:"zonePrefix"`
}

// emptyNames fills fields with non-nil values
func emptyNames() *names {
	return &names{
		IngressControllerNames:       []string{},
		CloudEphemeralNodeGroupNames: []string{},
		Zones:                        []string{},
	}
}

func (n *names) WithIngressControllers(ingNames ...string) *names {
	n.IngressControllerNames = ingNames
	return n
}

func (n *names) WithZones(zones ...string) *names {
	n.Zones = zones
	return n
}

func (n *names) WithZonePrefix(p string) *names {
	n.ZonePrefix = p
	return n
}

func (n *names) WithNodeGroups(ngNames ...string) *names {
	n.CloudEphemeralNodeGroupNames = ngNames
	return n
}
