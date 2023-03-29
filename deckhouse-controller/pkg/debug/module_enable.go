package debug

import (
	"fmt"

	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	deckhouse_config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

func DefineModuleConfigDebugCommands(kpApp *kingpin.Application) {
	moduleCmd := kpApp.GetCommand("module")

	var moduleName string
	moduleEnableCmd := moduleCmd.Command("enable", "Enable module via spec.enabled flag in ModuleConfig resource.").
		Action(func(c *kingpin.ParseContext) error {
			return moduleSwitch(moduleName, false, "disable")
		})
	moduleEnableCmd.Arg("module_name", "").Required().StringVar(&moduleName)

	moduleDisableCmd := moduleCmd.Command("disable", "Disable module via spec.enabled flag in ModuleConfig resource.").
		Action(func(c *kingpin.ParseContext) error {
			return moduleSwitch(moduleName, false, "disable")
		})
	moduleDisableCmd.Arg("module_name", "").Required().StringVar(&moduleName)
}

func moduleSwitch(moduleName string, enabled bool, actionDesc string) error {
	// Init logging for console output.
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true, ForceColors: true})

	// Init Kubernetes client.
	kubeClient := shell_operator.DefaultMainKubeClient(nil, nil)
	err := kubeClient.Init()
	if err != nil {
		return err
	}

	err = deckhouse_config.SetModuleConfigEnabledFlag(kubeClient, moduleName, enabled)
	if err != nil {
		return fmt.Errorf("%s module failed: %v", actionDesc, err)
	}
	return nil
}
