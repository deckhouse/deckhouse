// Copyright 2022 Flant JSC
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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	internalschema "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/pkg/schema"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// ensure crds hook has order 5, for creating node group we should use greater number
	OnStartup: &go_hook.OrderedConfig{Order: 6},
}, createMasterNodeGroup)

var defaultMasterNodeGroup = internalv1.NodeGroup{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "deckhouse.io/v1",
		Kind:       "NodeGroup",
	},

	ObjectMeta: metav1.ObjectMeta{
		Name: "master",
	},

	Spec: internalv1.NodeGroupSpec{
		NodeType: internalv1.NodeTypeCloudPermanent,
		Disruptions: internalv1.Disruptions{
			ApprovalMode: "Manual",
		},

		NodeTemplate: internalschema.NodeTemplate{
			Labels: map[string]string{
				"node-role.kubernetes.io/master":        "",
				"node-role.kubernetes.io/control-plane": "",
			},
			Taints: []v1.Taint{
				{
					Key:    "node-role.kubernetes.io/master",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
	},
}

func createMasterNodeGroup(input *go_hook.HookInput) error {
	input.PatchCollector.Create(&defaultMasterNodeGroup, object_patch.IgnoreIfExists())

	return nil
}
