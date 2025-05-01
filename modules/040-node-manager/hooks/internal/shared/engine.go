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

package shared

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/kube/object_patch"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/module"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	EngineMigrationSuccessfulConfigMapName = "d8-node-group-engine-migration"
	UseMCMAnnotationKey                    = "node.deckhouse.io/use-mcm"
)

var (
	ProvidersWithCAPIOnly = []string{
		"cloud-provider-dynamics",
		"cloud-provider-zvirt",
		"cloud-provider-vcd",
		"cloud-provider-huaweicloud",
	}

	ProvidersWithCAPIAnsMCM = []string{
		"cloud-provider-openstack",
	}
)

type NodeGroupEngineParam struct {
	Engine             ngv1.NodeGroupEngine
	NodeGroupName      string
	Type               ngv1.NodeType
	HasStaticInstances bool
	ShouldUseMCM       bool
}

func GetCloudEphemeralNodeGroupEngineDefault(input *go_hook.HookInput) ngv1.NodeGroupEngine {
	for _, provider := range ProvidersWithCAPIOnly {
		if module.IsEnabled(provider, input) {
			input.Logger.Infof("Enabling CAPI only for provider %s", provider)
			return ngv1.NodeGroupEngineCAPI
		}
	}
	for _, provider := range ProvidersWithCAPIAnsMCM {
		if module.IsEnabled(provider, input) {
			input.Logger.Infof("Enabling CAPI an MCM for provider %s", provider)
			return ngv1.NodeGroupEngineCAPI
		}
	}

	return ngv1.NodeGroupEngineMCM
}

func NodeGroupToNodeGroupEngineParam(ng *ngv1.NodeGroup) NodeGroupEngineParam {
	hasStaticInstances := false
	t := ng.Spec.NodeType
	if t == ngv1.NodeTypeStatic && ng.Spec.StaticInstances != nil {
		hasStaticInstances = true
	}

	shouldUseMCM := false
	annotations := ng.GetAnnotations()
	if annotations != nil {
		if _, ok := annotations[UseMCMAnnotationKey]; ok {
			shouldUseMCM = true
		}
	}

	return NodeGroupEngineParam{
		Engine:             ng.Status.Engine,
		NodeGroupName:      ng.GetName(),
		Type:               t,
		HasStaticInstances: hasStaticInstances,
		ShouldUseMCM:       shouldUseMCM,
	}
}

func CalculateEngine(ng NodeGroupEngineParam, cloudEphemeralDefault ngv1.NodeGroupEngine) (ngv1.NodeGroupEngine, error) {
	var engine ngv1.NodeGroupEngine
	switch ng.Type {
	case ngv1.NodeTypeStatic:
		if ng.HasStaticInstances {
			engine = ngv1.NodeGroupEngineCAPI
		} else {
			engine = ngv1.NodeGroupEngineNone
		}
	case ngv1.NodeTypeCloudStatic:
		engine = ngv1.NodeGroupEngineNone
	case ngv1.NodeTypeCloudPermanent:
		engine = ngv1.NodeGroupEngineNone
	case ngv1.NodeTypeCloudEphemeral:
		if ng.ShouldUseMCM {
			engine = ngv1.NodeGroupEngineMCM
		} else {
			engine = cloudEphemeralDefault
		}
	default:
		return "", fmt.Errorf("unknown node type %s for node group %s", ng.Type)
	}

	return engine, nil
}

func StatusEnginePatch(input *go_hook.HookInput, ngName string, engine ngv1.NodeGroupEngine) {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"engine": engine,
		},
	}

	input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "NodeGroup", "", ngName, object_patch.WithSubresource("/status"))
}
