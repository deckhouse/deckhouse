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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

var conversionLog = logf.Log.WithName("nodegroup-conversion-webhook")

// ProviderClusterConfiguration holds parsed provider config from Secret.
// This is used to determine CloudPermanent vs CloudStatic for Hybrid nodeType.
type ProviderClusterConfiguration struct {
	NodeGroups []ProviderNodeGroup `json:"nodeGroups" yaml:"nodeGroups"`
}

// ProviderNodeGroup represents a node group from provider config
type ProviderNodeGroup struct {
	Name string `json:"name" yaml:"name"`
}

// NodeGroupConversionHandler handles conversion requests with access to cluster state.
//
// This is needed because the standard conversion.Hub/Convertible interfaces
// don't have access to cluster state (Secrets, ConfigMaps).
//
// The Python hook in Deckhouse uses `includeSnapshotsFrom: ["cluster_config"]`
// to get provider configuration for determining CloudPermanent vs CloudStatic.
// We replicate this by reading the Secret directly.
type NodeGroupConversionHandler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// ServeHTTP implements http.Handler for the conversion webhook
func (h *NodeGroupConversionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		conversionLog.Error(err, "failed to read request body")
		h.writeError(w, "", "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Decode ConversionReview
	review := &apix.ConversionReview{}
	if err := json.Unmarshal(body, review); err != nil {
		conversionLog.Error(err, "failed to unmarshal conversion review")
		h.writeError(w, "", "failed to unmarshal conversion review")
		return
	}

	if review.Request == nil {
		conversionLog.Error(nil, "conversion review request is nil")
		h.writeError(w, "", "conversion review request is nil")
		return
	}

	conversionLog.Info("received conversion request",
		"uid", review.Request.UID,
		"desiredVersion", review.Request.DesiredAPIVersion,
		"objectCount", len(review.Request.Objects),
	)

	// Load provider config for Hybrid -> CloudPermanent/CloudStatic decision
	providerConfig, err := h.loadProviderConfig(ctx)
	if err != nil {
		conversionLog.Error(err, "failed to load provider config")
		h.writeError(w, string(review.Request.UID), fmt.Sprintf("failed to load provider config: %v", err))
		return
	}

	response := h.handleConversion(review.Request, providerConfig)
	review.Response = response
	review.Request = nil // Clear request in response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(review); err != nil {
		conversionLog.Error(err, "failed to encode conversion review response")
	}
}

// loadProviderConfig reads the provider cluster configuration from Secret.
// Returns empty config if secret is not found (expected for Static clusters).
// Returns error for transient failures (timeout, permission denied, etc.)
func (h *NodeGroupConversionHandler) loadProviderConfig(ctx context.Context) (*ProviderClusterConfiguration, error) {
	config := &ProviderClusterConfiguration{}

	secret := &corev1.Secret{}
	err := h.Client.Get(ctx, types.NamespacedName{
		Namespace: "kube-system",
		Name:      "d8-provider-cluster-configuration",
	}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Static cluster — secret doesn't exist, this is OK
			conversionLog.V(1).Info("provider config secret not found (expected for Static clusters)")
			return config, nil
		}
		// Timeout, permission denied, API unavailable — this is an error
		return nil, fmt.Errorf("failed to get secret kube-system/d8-provider-cluster-configuration: %w", err)
	}

	configData, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]
	if !ok {
		// Secret exists but doesn't have expected key — treat as empty config
		conversionLog.V(1).Info("cloud-provider-cluster-configuration.yaml not found in secret")
		return config, nil
	}

	if err := yaml.Unmarshal(configData, config); err != nil {
		return nil, fmt.Errorf("failed to parse cloud-provider-cluster-configuration.yaml: %w", err)
	}

	conversionLog.V(1).Info("loaded provider config", "nodeGroupsCount", len(config.NodeGroups))
	return config, nil
}

// handleConversion processes the conversion request
func (h *NodeGroupConversionHandler) handleConversion(req *apix.ConversionRequest, providerConfig *ProviderClusterConfiguration) *apix.ConversionResponse {
	response := &apix.ConversionResponse{
		UID:              req.UID,
		ConvertedObjects: make([]runtime.RawExtension, 0, len(req.Objects)),
		Result:           metav1.Status{Status: "Success"},
	}

	for i, obj := range req.Objects {
		converted, err := h.convertObject(obj.Raw, req.DesiredAPIVersion, providerConfig)
		if err != nil {
			conversionLog.Error(err, "failed to convert object", "index", i)
			response.Result = metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("failed to convert object %d: %v", i, err),
			}
			return response
		}
		response.ConvertedObjects = append(response.ConvertedObjects, runtime.RawExtension{Raw: converted})
	}

	return response
}

// convertObject converts a single object from source to desired version
func (h *NodeGroupConversionHandler) convertObject(raw []byte, desiredVersion string, providerConfig *ProviderClusterConfiguration) ([]byte, error) {
	var meta struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse object metadata: %w", err)
	}

	srcVersion := meta.APIVersion
	name := meta.Metadata.Name

	conversionLog.Info("converting object",
		"name", name,
		"from", srcVersion,
		"to", desiredVersion,
	)

	// If same version, return as-is
	if srcVersion == desiredVersion {
		return raw, nil
	}
	hubObj, err := h.convertToHub(raw, srcVersion, name, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to hub: %w", err)
	}
	return h.convertFromHub(hubObj, desiredVersion)
}

// convertToHub converts from any version to Hub (v1)
func (h *NodeGroupConversionHandler) convertToHub(raw []byte, srcVersion, name string, providerConfig *ProviderClusterConfiguration) (*v1.NodeGroup, error) {
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

// convertFromHub converts from Hub (v1) to any desired version
func (h *NodeGroupConversionHandler) convertFromHub(hub *v1.NodeGroup, desiredVersion string) ([]byte, error) {
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

// convertV1Alpha1ToHub converts v1alpha1 to v1 (Hub) with provider config logic
func (h *NodeGroupConversionHandler) convertV1Alpha1ToHub(src *v1alpha1.NodeGroup, name string, providerConfig *ProviderClusterConfiguration) (*v1.NodeGroup, error) {
	dst := &v1.NodeGroup{}
	if err := src.ConvertTo(dst); err != nil {
		return nil, err
	}
	h.overrideNodeTypeFromAlpha(v1alpha2.NodeType(src.Spec.NodeType), name, providerConfig, &dst.Spec.NodeType)
	return dst, nil
}

// convertV1Alpha2ToHub converts v1alpha2 to v1 (Hub) with provider config logic
func (h *NodeGroupConversionHandler) convertV1Alpha2ToHub(src *v1alpha2.NodeGroup, name string, providerConfig *ProviderClusterConfiguration) (*v1.NodeGroup, error) {
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
//   - master NodeGroup is always CloudPermanent
//   - NodeGroup found in providerConfig.nodeGroups is CloudPermanent
//   - Otherwise CloudStatic
func (h *NodeGroupConversionHandler) overrideNodeTypeFromAlpha(srcType v1alpha2.NodeType, name string, providerConfig *ProviderClusterConfiguration, dstType *v1.NodeType) {
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
func (h *NodeGroupConversionHandler) isCloudPermanent(name string, providerConfig *ProviderClusterConfiguration) bool {
	// master is always CloudPermanent
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

func (h *NodeGroupConversionHandler) writeError(w http.ResponseWriter, uid string, message string) {
	w.Header().Set("Content-Type", "application/json")

	review := &apix.ConversionReview{
		Response: &apix.ConversionResponse{
			UID: types.UID(uid),
			Result: metav1.Status{
				Status:  "Failure",
				Message: message,
			},
		},
	}
	if err := json.NewEncoder(w).Encode(review); err != nil {
		conversionLog.Error(err, "failed to encode error response")
	}
}
