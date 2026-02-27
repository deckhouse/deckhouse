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

package controller

import (
	capicontroller "github.com/deckhouse/node-controller/internal/controller/capi"
	instancecontroller "github.com/deckhouse/node-controller/internal/controller/instance"
	machinecontroller "github.com/deckhouse/node-controller/internal/controller/machine"
	nodecontroller "github.com/deckhouse/node-controller/internal/controller/node"
	ctrl "sigs.k8s.io/controller-runtime"
)

const MachineNamespace = machinecontroller.MachineNamespace

func SetupInstanceController(mgr ctrl.Manager) error {
	return instancecontroller.SetupInstanceController(mgr)
}

func SetupCAPIMachineController(mgr ctrl.Manager) error {
	return capicontroller.SetupCAPIMachineController(mgr)
}

func SetupNodeController(mgr ctrl.Manager) error {
	return nodecontroller.SetupNodeController(mgr)
}
