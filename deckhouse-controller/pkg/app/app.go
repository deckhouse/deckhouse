// Copyright 2025 Flant JSC
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

package app

import "os"

var Version = ""

const (
	ModuleDeckhouse = "deckhouse"
	ModuleGlobal    = "global"

	NamespaceDeckhouse  = "d8-system"
	NamespaceKubernetes = "kube-system"

	DiscoverySecret            = "deckhouse-discovery"
	ClusterConfigurationSecret = "d8-cluster-configuration"
	RegistrySecret             = "deckhouse-registry"

	EmbeddedModulesDir = "/deckhouse/modules"

	Name        = "deckhouse"
	Description = "controller for Kubernetes platform from Flant"

	PathToCRDs = "/deckhouse/deckhouse-controller/crds/*.yaml"

	LockName  = "deckhouse-bootstrap-lock"
	LeaseName = "deckhouse-leader-election"
)

var (
	ModeHA = os.Getenv("DECKHOUSE_HA") == "true"

	PodName      = os.Getenv("DECKHOUSE_POD")
	PodNamespace = os.Getenv("POD_NAMESPACE")
	PodIP        = os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")

	ClusterDomain = os.Getenv("KUBERNETES_CLUSTER_DOMAIN")

	Bundle = os.Getenv("DECKHOUSE_BUNDLE")

	NodeName = os.Getenv("DECKHOUSE_NODE_NAME")
)
