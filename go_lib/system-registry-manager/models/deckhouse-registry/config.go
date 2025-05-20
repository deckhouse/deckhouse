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

package deckhouseregistry

import (
	"encoding/base64"
)

type Config struct {
	Address        string
	Path           string
	Scheme         string
	CA             string
	ImagesRegistry string
	DockerConfig   []byte
}

func (cfg *Config) Equal(other *Config) bool {
	if other == nil {
		return false
	}
	return cfg.Address == other.Address &&
		cfg.Path == other.Path &&
		cfg.Scheme == other.Scheme &&
		cfg.CA == other.CA &&
		cfg.ImagesRegistry == other.ImagesRegistry &&
		string(cfg.DockerConfig) == string(other.DockerConfig)
}

func (cfg *Config) FromSecretData(data map[string][]byte) {
	*cfg = Config{
		Address:        string(data["address"]),
		Path:           string(data["path"]),
		Scheme:         string(data["scheme"]),
		CA:             string(data["ca"]),
		ImagesRegistry: string(data["imagesRegistry"]),
		DockerConfig:   data[".dockerconfigjson"],
	}
}

func (cfg *Config) ToBase64SecretData() map[string]string {
	return map[string]string{
		"address":           base64.StdEncoding.EncodeToString([]byte(cfg.Address)),
		"path":              base64.StdEncoding.EncodeToString([]byte(cfg.Path)),
		"scheme":            base64.StdEncoding.EncodeToString([]byte(cfg.Scheme)),
		"ca":                base64.StdEncoding.EncodeToString([]byte(cfg.CA)),
		"imagesRegistry":    base64.StdEncoding.EncodeToString([]byte(cfg.ImagesRegistry)),
		".dockerconfigjson": base64.StdEncoding.EncodeToString(cfg.DockerConfig),
	}
}
