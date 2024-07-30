// Copyright 2023 Flant JSC
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

package phases

import (
	"errors"
)

type (
	OperationPhase string
)

// Define common operations phases for such operations as bootstrap, converge and destroy.
// Notice that each operation could define own phases (like attach operation do).
const (
	BaseInfraPhase                         OperationPhase = "BaseInfra"
	RegistryPackagesProxyPhase             OperationPhase = "RegistryPackagesProxyBundle"
	ExecuteBashibleBundlePhase             OperationPhase = "ExecuteBashibleBundle"
	InstallDeckhousePhase                  OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase                   OperationPhase = "CreateResources"
	InstallAdditionalMastersAndStaticNodes OperationPhase = "InstallAdditionalMastersAndStaticNodes"
	DeleteResourcesPhase                   OperationPhase = "DeleteResources"
	ExecPostBootstrapPhase                 OperationPhase = "ExecPostBootstrap"
	AllNodesPhase                          OperationPhase = "AllNodes"
	FinalizationPhase                      OperationPhase = "Finalization"
)

var (
	StopOperationCondition = errors.New("StopOperationCondition")
)
