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
	"encoding/base64"
	"sort"
)

type CommonSettings struct {
	Name        string      `json:"-"`
	Type        string      `json:"type"`
	Inputs      []string    `json:"inputs,omitempty"`
	Healthcheck Healthcheck `json:"healthcheck"`
	Buffer      Buffer      `json:"buffer,omitempty"`
}

type Healthcheck struct {
	Enabled bool `json:"enabled"`
}

type CommonTLS struct {
	CAFile            string `json:"ca_file,omitempty"`
	CertFile          string `json:"crt_file,omitempty"`
	KeyFile           string `json:"key_file,omitempty"`
	KeyPass           string `json:"key_pass,omitempty"`
	VerifyHostname    bool   `json:"verify_hostname"`
	VerifyCertificate bool   `json:"verify_certificate"`
}

type Buffer struct {
	Size uint32 `json:"max_size,omitempty"`
	Type string `json:"type,omitempty"`
}

func (cs *CommonSettings) SetInputs(inp []string) {
	sort.Strings(inp)
	cs.Inputs = inp
}

func (cs *CommonSettings) GetName() string {
	return cs.Name
}

func decodeB64(input string) string {
	res, _ := base64.StdEncoding.DecodeString(input)
	return string(res)
}

func ComposeName(n string) string {
	return "destination/cluster/" + n
}
