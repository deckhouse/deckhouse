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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Config struct {
	Address      string `json:"address" yaml:"address"`
	Path         string `json:"path" yaml:"path"`
	Scheme       string `json:"scheme" yaml:"scheme"`
	CA           string `json:"ca,omitempty" yaml:"ca,omitempty"`
	DockerConfig []byte `json:".dockerconfigjson" yaml:".dockerconfigjson"`
}

func (cfg *Config) Equal(other *Config) bool {
	if other == nil {
		return false
	}
	return cfg.Address == other.Address &&
		cfg.Path == other.Path &&
		cfg.Scheme == other.Scheme &&
		cfg.CA == other.CA &&
		string(cfg.DockerConfig) == string(other.DockerConfig)
}

func (cfg *Config) FromSecretData(data map[string][]byte) {
	*cfg = Config{
		Address:      string(data["address"]),
		Path:         string(data["path"]),
		Scheme:       string(data["scheme"]),
		CA:           string(data["ca"]),
		DockerConfig: data[".dockerconfigjson"],
	}
}

func (cfg *Config) ToBase64SecretData() map[string]string {
	return map[string]string{
		"address":           base64.StdEncoding.EncodeToString([]byte(cfg.Address)),
		"path":              base64.StdEncoding.EncodeToString([]byte(cfg.Path)),
		"scheme":            base64.StdEncoding.EncodeToString([]byte(cfg.Scheme)),
		"ca":                base64.StdEncoding.EncodeToString([]byte(cfg.CA)),
		".dockerconfigjson": base64.StdEncoding.EncodeToString(cfg.DockerConfig),
	}
}

func (cfg *Config) Hash() (string, error) {
	buf, err := json.Marshal(*cfg)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	hash := sha256.New()
	hash.Write(buf)
	hashBytes := hash.Sum(nil)

	return hex.EncodeToString(hashBytes), nil
}
