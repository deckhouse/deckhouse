package app

import (
	"fmt"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	addonapp "github.com/flant/addon-operator/pkg/app"
	"os"
	"path/filepath"
	"runtime"
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
