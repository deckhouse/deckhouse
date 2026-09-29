// Copyright 2026 Flant JSC
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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	gatewayAPICRDsSnapshot = "gateway_api_crds"
	gatewayAPICRDsPath     = "global.discovery.gatewayAPICRDs"
)

// gatewayAPICRDs are the Gateway API CRDs whose presence is discovered. Every one of them is
// published, installed or not, so a consumer can look one up without first knowing which names
// exist; an absent CRD is the one with no versions.
var gatewayAPICRDs = []string{
	"backendtlspolicies.gateway.networking.k8s.io",
	"gatewayclasses.gateway.networking.k8s.io",
	"gateways.gateway.networking.k8s.io",
	"grpcroutes.gateway.networking.k8s.io",
	"httproutes.gateway.networking.k8s.io",
	"inferencepools.inference.networking.k8s.io",
	"listenersets.gateway.networking.k8s.io",
	"referencegrants.gateway.networking.k8s.io",
	"tcproutes.gateway.networking.k8s.io",
	"tlsroutes.gateway.networking.k8s.io",
	"udproutes.gateway.networking.k8s.io",
}

// gatewayAPICRD is one CRD and the versions it defines. Publishing the versions rather than a single
// "is it there" flag lets each consumer decide for itself which version it needs and whether being
// served is enough, instead of every new requirement needing another discovery field.
type gatewayAPICRD struct {
	Name     string                 `json:"name"`
	Versions []gatewayAPICRDVersion `json:"versions"`
}

// gatewayAPICRDVersion carries the fields of a CRD version that decide whether it can be used, under
// the names the CustomResourceDefinition itself uses. Deprecation is not among them: a deprecated
// version is dropped rather than reported, so a consumer never has to filter one out itself.
type gatewayAPICRDVersion struct {
	Name    string `json:"name"`
	Served  bool   `json:"served"`
	Storage bool   `json:"storage"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       gatewayAPICRDsSnapshot,
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: gatewayAPICRDs,
			},
			FilterFunc: applyGatewayAPICRDFilter,
		},
	},
}, setGatewayAPICRDs)

// applyGatewayAPICRDFilter returns the CRD's name and the versions it defines, less the deprecated
// ones. A CRD whose every version is deprecated is reported the same way as one that is absent.
func applyGatewayAPICRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := sdk.FromUnstructured(obj, &crd); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	versions := make([]gatewayAPICRDVersion, 0, len(crd.Spec.Versions))
	for _, version := range crd.Spec.Versions {
		if version.Deprecated {
			continue
		}
		versions = append(versions, gatewayAPICRDVersion{
			Name:    version.Name,
			Served:  version.Served,
			Storage: version.Storage,
		})
	}

	return gatewayAPICRD{Name: crd.Name, Versions: versions}, nil
}

func setGatewayAPICRDs(_ context.Context, input *go_hook.HookInput) error {
	installed, err := sdkobjectpatch.UnmarshalToStruct[gatewayAPICRD](input.Snapshots, gatewayAPICRDsSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", gatewayAPICRDsSnapshot, err)
	}

	versionsByCRD := make(map[string][]gatewayAPICRDVersion, len(installed))
	for _, crd := range installed {
		versionsByCRD[crd.Name] = crd.Versions
	}

	// Built from gatewayAPICRDs rather than from the snapshot, so the list holds every known CRD in
	// a fixed order however many are installed: a consumer scanning it never has to distinguish
	// "absent from the list" from "present with nothing served".
	available := make([]gatewayAPICRD, 0, len(gatewayAPICRDs))
	for _, name := range gatewayAPICRDs {
		versions := versionsByCRD[name]
		if versions == nil {
			versions = []gatewayAPICRDVersion{}
		}
		available = append(available, gatewayAPICRD{Name: name, Versions: versions})
	}

	input.Values.Set(gatewayAPICRDsPath, available)

	return nil
}
