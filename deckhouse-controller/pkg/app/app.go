// Copyright 2024 Flant JSC
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

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	addonapp "github.com/flant/addon-operator/pkg/app"

	"github.com/deckhouse/deckhouse/go_lib/d8env"
)

const (
	Name        = "deckhouse"
	Description = "controller for Kubernetes platform from Flant"

	// VersionDeckhouse is set by 'go build' command.
	VersionDeckhouse = "dev"
	// VersionAddonOperator is set by 'go build' command.
	VersionAddonOperator = "dev"
	// VersionShellOperator is set by 'go build' command.
	VersionShellOperator = "dev"

	ModuleDeckhouse = "deckhouse"
	ModuleGlobal    = "global"

	NamespaceDeckhouse  = "d8-system"
	NamespaceKubernetes = "kube-system"

	ClusterConfigurationSecret = "d8-cluster-configuration"
)

var (
	TestVarExtenderBootstrapped      = os.Getenv("TEST_EXTENDER_BOOTSTRAPPED")
	TestVarExtenderDeckhouseVersion  = os.Getenv("TEST_EXTENDER_DECKHOUSE_VERSION")
	TestVarExtenderKubernetesVersion = os.Getenv("TEST_EXTENDER_KUBERNETES_VERSION")

	VarBundle   = os.Getenv("DECKHOUSE_BUNDLE")
	VarNodeName = os.Getenv("DECKHOUSE_NODE_NAME")
	VarModeHA   = os.Getenv("DECKHOUSE_HA")

	VarModulesDirs          = addonapp.ModulesDir
	VarGlobalHooksDir       = addonapp.GlobalHooksDir
	VarEmbeddedModulesDir   = "/modules"
	VarDownloadedModulesDir = d8env.GetDownloadedModulesDir()
	VarSymlinksModulesDir   = filepath.Join(d8env.GetDownloadedModulesDir(), "modules")
)

func Version() string {
	return fmt.Sprintf("deckhouse %s (addon-operator %s, shell-operator %s, Golang %s)", VersionDeckhouse, VersionAddonOperator, VersionShellOperator, runtime.Version())
}
