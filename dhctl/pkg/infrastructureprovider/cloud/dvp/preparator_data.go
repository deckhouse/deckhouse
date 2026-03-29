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

package dvp

import (
	"fmt"

	"github.com/name212/govalue"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type PreparatorAdditionalData struct {
	originalProviderClusterConfigYAML string
}

func NewPreparatorAdditionalData(origProviderConfigYAML string) *PreparatorAdditionalData {
	return &PreparatorAdditionalData{
		originalProviderClusterConfigYAML: origProviderConfigYAML,
	}
}

func PreparatorAdditionalDataFromAny(data any) (*PreparatorAdditionalData, error) {
	if govalue.IsNil(data) {
		return nil, nil
	}

	res, ok := data.(*PreparatorAdditionalData)
	if !ok {
		return nil, fmt.Errorf("Internal error. Additional data is not *PreparatorAdditionalData for cloud provider dvp. Got %T", data)
	}

	return res, nil
}

func (d *PreparatorAdditionalData) logSkip(logger log.Logger, msg string) (string, error) {
	logger.LogDebugF("%s. Skip\n", msg)
	return "", nil
}

func (d *PreparatorAdditionalData) extractSSHPubKey(logger log.Logger) (string, error) {
	// Warning! Do not trim config!

	if d.originalProviderClusterConfigYAML == "" {
		return d.logSkip(logger, "Original provider cluster config yaml key not provided")
	}

	type providerConfiguration struct {
		SSHPubKey *string `json:"sshPublicKey,omitempty"`
	}

	providerConfig := providerConfiguration{}

	err := yaml.Unmarshal([]byte(d.originalProviderClusterConfigYAML), &providerConfig)
	if err != nil {
		return "", fmt.Errorf("Cannot unmarshal original provider cluster config: %w", err)
	}

	if providerConfig.SSHPubKey == nil {
		return d.logSkip(logger, "Original provider cluster config does not contains ssh pub key")
	}

	return *providerConfig.SSHPubKey, nil
}
