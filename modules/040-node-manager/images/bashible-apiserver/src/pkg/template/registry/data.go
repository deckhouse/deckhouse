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

package registry

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
)

type RegistryData bashible.Context

func (rd *RegistryData) loadFromInput(deckhouseRegistrySecret deckhouseRegistrySecret, bashibleCfgSecret *bashibleConfigSecret) error {
	if bashibleCfgSecret != nil {
		rData := bashibleCfgSecret.toRegistryData()
		*rd = *rData
		return nil
	}

	rData, err := deckhouseRegistrySecret.toRegistryData()
	if err != nil {
		return err
	}
	*rd = *rData
	return nil
}

func (rd *RegistryData) hashSum() (string, error) {
	data, err := json.Marshal(rd)
	if err != nil {
		return "", fmt.Errorf("error marshalling data: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}

func (rd *RegistryData) validate() error {
	if rd == nil {
		return fmt.Errorf("failed: is empty")
	}
	ctx := bashible.Context(*rd)
	return ctx.Validate()
}

func (rd *RegistryData) toMap() (map[string]interface{}, error) {
	if rd == nil {
		return nil, fmt.Errorf("failed: is empty")
	}
	ctx := bashible.Context(*rd)
	return ctx.ToMap()
}
