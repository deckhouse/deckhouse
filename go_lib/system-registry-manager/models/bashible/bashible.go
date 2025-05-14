/*
Copyright 2025 Flant JSC

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

package bashible

import (
	"encoding/json"

	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

type BashibleContext struct {
	Mode           registry_const.ModeType `json:"mode" yaml:"mode"`
	Version        string                  `json:"version" yaml:"version"`
	ImagesBase     string                  `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string                `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []HostsObject           `json:"hosts" yaml:"hosts"`
	PrepullHosts   []HostsObject           `json:"prepullHosts" yaml:"prepullHosts"`
}

type BashibleConfigSecret struct {
	Mode           registry_const.ModeType `json:"mode" yaml:"mode"`
	Version        string                  `json:"version" yaml:"version"`
	ImagesBase     string                  `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string                `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []HostsObject           `json:"hosts" yaml:"hosts"`
	PrepullHosts   []HostsObject           `json:"prepullHosts" yaml:"prepullHosts"`
}

type HostsObject struct {
	Host    string             `json:"host" yaml:"host"`
	CA      []string           `json:"ca" yaml:"ca"`
	Mirrors []MirrorHostObject `json:"mirrors" yaml:"mirrors"`
}

type MirrorHostObject struct {
	Host     string `json:"host" yaml:"host"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
	Scheme   string `json:"scheme" yaml:"scheme"`
}

func ToMap(s interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	return result, err
}
