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

package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func (pc *Checker) CheckYandexWithNatInstanceConfig(_ context.Context) error {
	if app.PreflightSkipYandexWithNatInstanceCheck {
		log.DebugLn("Yandex NAT instance config check is skipped")
		return nil
	}

	configObject := make(map[string]any)
	configKind, err := unmarshalProviderClusterConfiguration(pc.installConfig.ProviderClusterConfig, configObject)
	if err != nil {
		return fmt.Errorf("unmarshal provider cluster configuration: %v", err)
	}

	if configKind != "YandexClusterConfiguration" {
		log.DebugLn("cluster configuration provider is not Yandex, skipping")
		return nil
	}

	layout, found := configObject["layout"]
	if !found {
		return errors.New("layout not found in provider cluster configuration")
	}

	if layout != "WithNATInstance" {
		log.DebugLn("layout is not WithNATInstance, skipping")
		return nil
	}

	withNATInstance, found := configObject["withNATInstance"]
	if !found {
		return errors.New("withNATInstance not found in provider cluster configuration")
	}

	_, foundInternalSubnetCIDR := withNATInstance.(map[string]interface{})["internalSubnetCIDR"]
	_, foundInternalSubnetID := withNATInstance.(map[string]interface{})["internalSubnetID"]
	if !foundInternalSubnetCIDR && !foundInternalSubnetID {
		return errors.New("neither internalSubnetCIDR nor internalSubnetID are provided")
	}

	return nil
}

func readPropertyAtPathAsString(configObject map[string]any, propertyPath ...string) (string, error) {
	if len(propertyPath) == 0 {
		return "", nil
	}

	propertyValue, found, err := unstructured.NestedFieldNoCopy(configObject, propertyPath...)
	if err != nil {
		return "", fmt.Errorf("malformed provider cluster configuration: reading .%s: %w", strings.Join(propertyPath, "."), err)
	}
	if !found {
		return "", fmt.Errorf("malformed provider cluster configuration: reading .%s: no such property", strings.Join(propertyPath, "."))
	}

	return propertyValue.(string), nil
}
