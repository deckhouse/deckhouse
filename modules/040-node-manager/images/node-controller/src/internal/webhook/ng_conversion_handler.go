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

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

// convertNGToHub converts NodeGroup from any supported version to the hub (v1).
func (h *ConversionHandler) convertNGToHub(raw []byte, srcVersion, name string, providerConfig *ProviderClusterConfiguration) (*v1.NodeGroup, error) {
	switch srcVersion {
	case "deckhouse.io/v1":
		obj := &v1.NodeGroup{}
		if err := json.Unmarshal(raw, obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal v1: %w", err)
		}
		return obj, nil

	case "deckhouse.io/v1alpha1":
		srcObj := &v1alpha1.NodeGroup{}
		if err := json.Unmarshal(raw, srcObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal v1alpha1: %w", err)
		}
		return h.convertV1Alpha1ToHub(srcObj, name, providerConfig)

	case "deckhouse.io/v1alpha2":
		srcObj := &v1alpha2.NodeGroup{}
		if err := json.Unmarshal(raw, srcObj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal v1alpha2: %w", err)
		}
		return h.convertV1Alpha2ToHub(srcObj, name, providerConfig)

	default:
		return nil, fmt.Errorf("unsupported source version: %s", srcVersion)
	}
}

// convertNGFromHub converts NodeGroup from hub (v1) to any supported version.
func (h *ConversionHandler) convertNGFromHub(hub *v1.NodeGroup, desiredVersion string) ([]byte, error) {
	var result interface{}

	switch desiredVersion {
	case "deckhouse.io/v1":
		hub.APIVersion = "deckhouse.io/v1"
		hub.Kind = "NodeGroup"
		result = hub

	case "deckhouse.io/v1alpha1":
		dstObj := &v1alpha1.NodeGroup{}
		if err := dstObj.ConvertFrom(hub); err != nil {
			return nil, fmt.Errorf("failed to convert v1 to v1alpha1: %w", err)
		}
		dstObj.APIVersion = "deckhouse.io/v1alpha1"
		dstObj.Kind = "NodeGroup"
		result = dstObj

	case "deckhouse.io/v1alpha2":
		dstObj := &v1alpha2.NodeGroup{}
		if err := dstObj.ConvertFrom(hub); err != nil {
			return nil, fmt.Errorf("failed to convert v1 to v1alpha2: %w", err)
		}
		dstObj.APIVersion = "deckhouse.io/v1alpha2"
		dstObj.Kind = "NodeGroup"
		result = dstObj

	default:
		return nil, fmt.Errorf("unsupported desired version: %s", desiredVersion)
	}

	return json.Marshal(result)
}

// convertV1Alpha1ToHub converts v1alpha1 to v1 (Hub) with provider config logic.
func (h *ConversionHandler) convertV1Alpha1ToHub(src *v1alpha1.NodeGroup, name string, providerConfig *ProviderClusterConfiguration) (*v1.NodeGroup, error) {
	dst := &v1.NodeGroup{}
	if err := src.ConvertTo(dst); err != nil {
		return nil, err
	}
	h.overrideNodeTypeFromAlpha(v1alpha2.NodeType(src.Spec.NodeType), name, providerConfig, &dst.Spec.NodeType)
	return dst, nil
}

// convertV1Alpha2ToHub converts v1alpha2 to v1 (Hub) with provider config logic.
func (h *ConversionHandler) convertV1Alpha2ToHub(src *v1alpha2.NodeGroup, name string, providerConfig *ProviderClusterConfiguration) (*v1.NodeGroup, error) {
	dst := &v1.NodeGroup{}
	if err := src.ConvertTo(dst); err != nil {
		return nil, err
	}
	h.overrideNodeTypeFromAlpha(src.Spec.NodeType, name, providerConfig, &dst.Spec.NodeType)

	return dst, nil
}

// overrideNodeTypeFromAlpha applies the correct nodeType mapping with provider config.
//
// This replicates the Python hook logic for Hybrid -> CloudPermanent/CloudStatic:
// - master NodeGroup is always CloudPermanent
// - NodeGroup found in providerConfig.nodeGroups is CloudPermanent
// - Otherwise CloudStatic
func (h *ConversionHandler) overrideNodeTypeFromAlpha(srcType v1alpha2.NodeType, name string, providerConfig *ProviderClusterConfiguration, dstType *v1.NodeType) {
	switch srcType {
	case v1alpha2.NodeTypeCloud:
		*dstType = v1.NodeTypeCloudEphemeral

	case v1alpha2.NodeTypeStatic:
		*dstType = v1.NodeTypeStatic

	case v1alpha2.NodeTypeHybrid:
		if h.isCloudPermanent(name, providerConfig) {
			*dstType = v1.NodeTypeCloudPermanent
			conversionLog.Info("mapped Hybrid to CloudPermanent", "name", name)
		} else {
			*dstType = v1.NodeTypeCloudStatic
			conversionLog.Info("mapped Hybrid to CloudStatic", "name", name)
		}

	default:
		conversionLog.V(1).Info("keeping existing nodeType mapping", "name", name, "srcType", srcType, "dstType", *dstType)
	}
}

// isCloudPermanent checks if the NodeGroup should be CloudPermanent based on provider config.
func (h *ConversionHandler) isCloudPermanent(name string, providerConfig *ProviderClusterConfiguration) bool {
	if name == "master" {
		return true
	}
	for _, ng := range providerConfig.NodeGroups {
		if ng.Name == name {
			return true
		}
	}

	return false
}
