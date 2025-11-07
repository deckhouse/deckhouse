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

package init

import (
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/users"
)

type Config struct {
	CA     *CertKey    `json:"ca,omitempty" yaml:"ca,omitempty"`
	UserRW *users.User `json:"userRW,omitempty" yaml:"userRW,omitempty"`
	UserRO *users.User `json:"userRO,omitempty" yaml:"userRO,omitempty"`
}

type CertKey struct {
	Cert string `json:"cert" yaml:"cert"`
	Key  string `json:"key" yaml:"key"`
}

func (c Config) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Config: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Config into map: %w", err)
	}
	return result, nil
}
