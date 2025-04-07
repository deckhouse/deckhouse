/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
