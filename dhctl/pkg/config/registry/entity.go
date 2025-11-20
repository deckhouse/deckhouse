// Copyright 2025 Flant JSC
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

package registry

import (
	"strings"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type InitConfig struct {
	ImagesRepo        string `json:"imagesRepo" yaml:"imagesRepo"`
	RegistryScheme    string `json:"registryScheme" yaml:"registryScheme"`
	RegistryDockerCfg string `json:"registryDockerCfg,omitempty" yaml:"registryDockerCfg,omitempty"`
	RegistryCA        string `json:"registryCA,omitempty" yaml:"registryCA,omitempty"`
}

type DeckhouseSettings struct {
	Mode      registry_const.ModeType `json:"mode" yaml:"mode"`
	Direct    *DirectModeSettings     `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *UnmanagedModeSettings  `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
}

type DirectModeSettings struct {
	ImagesRepo string                       `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     SchemeType                   `json:"scheme" yaml:"scheme"`
	CA         string                       `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string                       `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string                       `json:"password,omitempty" yaml:"password,omitempty"`
	License    string                       `json:"license,omitempty" yaml:"license,omitempty"`
	CheckMode  registry_const.CheckModeType `json:"checkMode,omitempty" yaml:"checkMode,omitempty"`
}

type UnmanagedModeSettings struct {
	ImagesRepo string                       `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     SchemeType                   `json:"scheme" yaml:"scheme"`
	CA         string                       `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string                       `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string                       `json:"password,omitempty" yaml:"password,omitempty"`
	License    string                       `json:"license,omitempty" yaml:"license,omitempty"`
	CheckMode  registry_const.CheckModeType `json:"checkMode,omitempty" yaml:"checkMode,omitempty"`
}

type SchemeType = string
type CRIType = string

func schemeFromString(scheme string) SchemeType {
	if strings.EqualFold(scheme, SchemeHTTP) {
		return SchemeHTTP
	}
	return SchemeHTTPS
}
