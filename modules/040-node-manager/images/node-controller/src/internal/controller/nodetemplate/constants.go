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

package nodetemplate

import nodecommon "github.com/deckhouse/node-controller/internal/common"

const (
	controllerName = "node-template"
	allRequestName = "__all__"

	nodeGroupNameLabel                = nodecommon.NodeGroupLabel
	lastAppliedNodeTemplateAnnotation = "node-manager.deckhouse.io/last-applied-node-template"
	nodeUninitializedTaintKey         = "node.deckhouse.io/uninitialized"
	masterNodeRoleKey                 = "node-role.kubernetes.io/master"
	clusterAPIAnnotationKey           = "cluster.x-k8s.io/machine"
	heartbeatAnnotationKey            = "kubevirt.internal.virtualization.deckhouse.io/heartbeat"
	metalLBmemberLabelKey             = "l2-load-balancer.network.deckhouse.io/member"
	controlPlaneTaintKey              = "node-role.kubernetes.io/control-plane"
)
