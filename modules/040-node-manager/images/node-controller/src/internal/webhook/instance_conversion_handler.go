/*
Copyright 2026 Flant JSC

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

package webhook

import (
	"encoding/json"
	"fmt"

	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

// convertInstanceToHub converts Instance from any supported version to the hub (v1alpha2).
func (h *ConversionHandler) convertInstanceToHub(raw []byte, srcVersion string) (*v1alpha2.Instance, error) {
	switch srcVersion {
	case "deckhouse.io/v1alpha2":
		obj := &v1alpha2.Instance{}
		if err := json.Unmarshal(raw, obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Instance v1alpha2: %w", err)
		}
		return obj, nil

	case "deckhouse.io/v1alpha1":
		srcObj := &v1alpha1.Instance{}
		if err := json.Unmarshal(raw, srcObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Instance v1alpha1: %w", err)
		}

		dstObj := &v1alpha2.Instance{}
		if err := srcObj.ConvertTo(dstObj); err != nil {
			return nil, fmt.Errorf("failed to convert Instance v1alpha1 to v1alpha2: %w", err)
		}
		return dstObj, nil

	default:
		return nil, fmt.Errorf("unsupported source version for Instance: %s", srcVersion)
	}
}

// convertInstanceFromHub converts Instance from hub (v1alpha2) to any supported version.
func (h *ConversionHandler) convertInstanceFromHub(hub *v1alpha2.Instance, desiredVersion string) ([]byte, error) {
	var result interface{}

	switch desiredVersion {
	case "deckhouse.io/v1alpha2":
		hub.APIVersion = "deckhouse.io/v1alpha2"
		hub.Kind = "Instance"
		result = hub

	case "deckhouse.io/v1alpha1":
		dstObj := &v1alpha1.Instance{}
		if err := dstObj.ConvertFrom(hub); err != nil {
			return nil, fmt.Errorf("failed to convert Instance v1alpha2 to v1alpha1: %w", err)
		}
		dstObj.APIVersion = "deckhouse.io/v1alpha1"
		dstObj.Kind = "Instance"
		result = dstObj

	default:
		return nil, fmt.Errorf("unsupported desired version for Instance: %s", desiredVersion)
	}

	return json.Marshal(result)
}
