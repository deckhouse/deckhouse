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
	"encoding/base64"
	"fmt"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	init_secret "github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

type (
	SecretData map[string][]byte

	// Bashible
	BashibleContext = bashible.Context
	BashibleConfig  = bashible.Config

	// PKI:
	PKI         = init_secret.Config
	PKICertKey  = init_secret.CertKey
	PKIProvider func() (PKI, error)
)

type KubeadmContext struct {
	Address string `json:"address" yaml:"address"`
	Path    string `json:"path" yaml:"path"`
}

func (ctx KubeadmContext) ToMap() map[string]any {
	return map[string]any{
		"address": ctx.Address,
		"path":    ctx.Path,
	}
}

type Data struct {
	ImagesRepo string              `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     constant.SchemeType `json:"scheme" yaml:"scheme"`
	CA         string              `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string              `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string              `json:"password,omitempty" yaml:"password,omitempty"`
}

func (d *Data) fromRegistrySettings(settings module_config.RegistrySettings) {
	*d = Data{
		ImagesRepo: settings.ImagesRepo,
		Scheme:     settings.Scheme,
		CA:         settings.CA,
		Username:   settings.Username,
		Password:   settings.Password,
	}

	if settings.License != "" {
		d.Username = constant.LicenseUsername
		d.Password = settings.License
	}
}

func (d Data) AuthBase64() string {
	if d.Username == "" {
		return ""
	}

	auth := fmt.Sprintf("%s:%s", d.Username, d.Password)
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (d Data) DockerCfg() ([]byte, error) {
	address, _ := d.AddressAndPath()
	return helpers.DockerCfgFromCreds(d.Username, d.Password, address)
}

func (d Data) DockerCfgBase64() (string, error) {
	cfg, err := d.DockerCfg()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cfg), nil
}

func (d Data) AddressAndPath() (address string, path string) {
	return helpers.SplitAddressAndPath(d.ImagesRepo)
}

func (d *Data) DeepCopyInto(out *Data) {
	*out = *d
}

func (d *Data) DeepCopy() *Data {
	if d == nil {
		return nil
	}
	out := new(Data)
	d.DeepCopyInto(out)
	return out
}
