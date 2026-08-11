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

package nodebootstrap

import "time"

const (
	controllerName = "node-bootstrap"

	// ownerWaitInterval is the requeue period while waiting for the owner ref
	// the MachineSet sets or the node-group label CAPI copies; both arrive
	// within seconds, the requeue is only a backstop for a missed event.
	ownerWaitInterval = 10 * time.Second

	// bootstrapRefreshInterval is how often userdata of a machine without a
	// node is re-rendered: the embedded bootstrap token lives four hours, so a
	// machine whose VM is created late must not keep its first copy.
	bootstrapRefreshInterval = 10 * time.Minute

	// kubeSystemNS holds the per-group bootstrap-token secrets.
	kubeSystemNS = "kube-system"

	// bootstrapTokenNGLabel labels a bootstrap-token secret with the NodeGroup
	// it belongs to.
	bootstrapTokenNGLabel = "node-manager.deckhouse.io/node-group"

	// machineNodeGroupLabel is the NodeGroup a Machine and the bootstrap config
	// cloned for it belong to. node-controller stamps it on the MachineDeployment
	// template, so CAPI copies it onto every Machine and every clone.
	machineNodeGroupLabel = "node-group"

	// nodeConfigPath is where the on-node loader reads its config from.
	nodeConfigPath = "/config/nodeconfig.yaml"

	// dataSecretSuffix names the Secret holding a machine's bootstrap userdata.
	dataSecretSuffix = "-bootstrap-data"

	// secretValueKey is the key the infrastructure provider (capdvp) reads the
	// userdata from — do not rename, capdvp reads Data["value"]. secretFormatKey
	// tells it the userdata is a cloud-config document.
	secretValueKey          = "value"
	secretFormatKey         = "format"
	secretFormatCloudConfig = "cloud-config"

	// machineKind and nodeBootstrapConfigKind name the owner references the
	// controller reads and writes.
	machineKind             = "Machine"
	nodeBootstrapConfigKind = "NodeBootstrapConfig"

	// conditionDataSecretAvailable reports whether the bootstrap userdata is
	// rendered and ready for the infrastructure provider to consume, and why not
	// when it is not.
	conditionDataSecretAvailable = "DataSecretAvailable"
	reasonRendered               = "Rendered"
	reasonWaitingForOwner        = "WaitingForOwnerMachine"
	reasonNodeGroupUnusable      = "NodeGroupUnusable"
	reasonRenderFailed           = "RenderFailed"
)
