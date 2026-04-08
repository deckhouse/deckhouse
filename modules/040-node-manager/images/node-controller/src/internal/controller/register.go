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
	_ "github.com/deckhouse/node-controller/internal/controller/controlplane"
	_ "github.com/deckhouse/node-controller/internal/controller/csinode"
	_ "github.com/deckhouse/node-controller/internal/controller/csr"
	_ "github.com/deckhouse/node-controller/internal/controller/deployment/bashiblelock"
	_ "github.com/deckhouse/node-controller/internal/controller/instance"
	_ "github.com/deckhouse/node-controller/internal/controller/machine/ycpreemptible"
	_ "github.com/deckhouse/node-controller/internal/controller/machinedeployment"
	_ "github.com/deckhouse/node-controller/internal/controller/metrics/caps"
	_ "github.com/deckhouse/node-controller/internal/controller/metrics/cloudconditions"
	_ "github.com/deckhouse/node-controller/internal/controller/metrics/containerd"
	_ "github.com/deckhouse/node-controller/internal/controller/metrics/nodegroupconfigurations"
	_ "github.com/deckhouse/node-controller/internal/controller/metrics/osversion"
	_ "github.com/deckhouse/node-controller/internal/controller/node/bashiblecleanup"
	_ "github.com/deckhouse/node-controller/internal/controller/node/fencing"
	_ "github.com/deckhouse/node-controller/internal/controller/node/gpu"
	_ "github.com/deckhouse/node-controller/internal/controller/node/providerid"
	_ "github.com/deckhouse/node-controller/internal/controller/node/template"
	_ "github.com/deckhouse/node-controller/internal/controller/node/update"
	_ "github.com/deckhouse/node-controller/internal/controller/nodegroup/chaosmonkey"
	_ "github.com/deckhouse/node-controller/internal/controller/nodegroup/instanceclass"
	_ "github.com/deckhouse/node-controller/internal/controller/nodegroup/master"
	_ "github.com/deckhouse/node-controller/internal/controller/nodegroup/status"
	_ "github.com/deckhouse/node-controller/internal/controller/nodeuser"
	_ "github.com/deckhouse/node-controller/internal/controller/pod/bashible"
	_ "github.com/deckhouse/node-controller/internal/controller/secret/crdwebhook"
)
