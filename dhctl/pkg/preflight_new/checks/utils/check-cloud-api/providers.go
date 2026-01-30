// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checkcloudapi

import (
	"encoding/json"
	"fmt"
)

func HandleOpenStackProvider(providerClusterConfig []byte) (*CloudApiConfig, error) {
	var openStackConfig OpenStackProvider
	err := json.Unmarshal(providerClusterConfig, &openStackConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider config for OpenStack: %v", err)
	}

	url, err := urlParse(openStackConfig.AuthURL)
	if err != nil {
		return nil, err
	}

	return &CloudApiConfig{
		URL:      url,
		CACert:   openStackConfig.CACert,
		Insecure: false,
	}, nil
}

func HandleVSphereProvider(providerClusterConfig []byte) (*CloudApiConfig, error) {
	var vsphereConfig VSphereProvider
	err := json.Unmarshal(providerClusterConfig, &vsphereConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider config for OpenStack: %v", err)
	}

	url, err := urlParse(vsphereConfig.Server)
	if err != nil {
		return nil, err
	}

	return &CloudApiConfig{
		URL:      url,
		CACert:   "",
		Insecure: vsphereConfig.Insecure,
	}, nil
}
