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

package debug

import (
	"fmt"

	"github.com/flant/kube-client/client"
	"gopkg.in/alecthomas/kingpin.v2"

	deckhouse_config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func DefineModuleConfigDebugCommands(kpApp *kingpin.Application, logger *log.Logger) {
	moduleCmd := kpApp.GetCommand("module")

	var moduleName string
	moduleEnableCmd := moduleCmd.Command("enable", "Enable module via spec.enabled flag in the ModuleConfig resource. Use snake-case for the module name.").
		Action(func(_ *kingpin.ParseContext) error {
			logger.SetLevel(log.LevelError)
			cli := client.New()
			err := cli.Init()
			if err != nil {
				return err
			}

			return moduleSwitch(cli, moduleName, true, "enable", logger)
		})
	moduleEnableCmd.Arg("module_name", "").Required().StringVar(&moduleName)

	moduleDisableCmd := moduleCmd.Command("disable", "Disable module via spec.enabled flag in the ModuleConfig resource. Use snake-case for the module name.").
		Action(func(_ *kingpin.ParseContext) error {
			logger.SetLevel(log.LevelError)
			cli := client.New()
			err := cli.Init()
			if err != nil {
				return err
			}

			return moduleSwitch(cli, moduleName, false, "disable", logger)
		})
	moduleDisableCmd.Arg("module_name", "").Required().StringVar(&moduleName)
}

func moduleSwitch(kubeClient *client.Client, moduleName string, enabled bool, actionDesc string, logger *log.Logger) error {
	// Init logging for console output.

	// TODO: check formatters?
	// log.SetFormatter(&log.TextFormatter{DisableTimestamp: true, ForceColors: true})
	logger.SetLevel(log.LevelError)

	err := deckhouse_config.SetModuleConfigEnabledFlag(kubeClient, moduleName, enabled)
	if err != nil {
		return fmt.Errorf("%s module failed: %v", actionDesc, err)
	}
	fmt.Printf("Module %s %sd\n", moduleName, actionDesc)
	return nil
}
