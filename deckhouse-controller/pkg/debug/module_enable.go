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
	"context"
	"fmt"

	"github.com/flant/addon-operator/sdk"
	"github.com/flant/kube-client/client"
	"gopkg.in/alecthomas/kingpin.v2"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func DefineModuleConfigDebugCommands(kpApp *kingpin.Application, logger *log.Logger) {
	moduleCmd := kpApp.GetCommand("module")

	var moduleName string
	moduleEnableCmd := moduleCmd.Command("enable", "Enable module via spec.enabled flag in the ModuleConfig resource. Use snake-case for the module name.").
		Action(func(_ *kingpin.ParseContext) error {
			logger.SetLevel(log.LevelError)
			cli := client.New(client.WithLogger(logger))
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
			cli := client.New(client.WithLogger(logger))
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

	if err := setModuleConfigEnabled(kubeClient, moduleName, enabled); err != nil {
		return fmt.Errorf("%s module failed: %v", actionDesc, err)
	}
	fmt.Printf("Module %s %sd\n", moduleName, actionDesc)
	return nil
}

// setModuleConfigEnabled updates spec.enabled flag or creates a new ModuleConfig with spec.enabled flag.
func setModuleConfigEnabled(kubeClient k8s.Client, name string, enabled bool) error {
	// this should not happen, but check it anyway.
	if kubeClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	unstructuredObj, err := kubeClient.Dynamic().Resource(v1alpha1.ModuleConfigGVR).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && !k8errors.IsNotFound(err) {
		return fmt.Errorf("failed to get the '%s' module config: %v", name, err)
	}

	if unstructuredObj != nil {
		if err = unstructured.SetNestedField(unstructuredObj.Object, enabled, "spec", "enabled"); err != nil {
			return fmt.Errorf("failed to change spec.enabled to %v in the '%s' module config/: %v", enabled, name, err)
		}
		if _, err = kubeClient.Dynamic().Resource(v1alpha1.ModuleConfigGVR).Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update the '%s' module config: %v", name, err)
		}
		return nil
	}

	// create new ModuleConfig if absent.
	newCfg := &v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleConfigGVK.Kind,
			APIVersion: v1alpha1.ModuleConfigGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled: ptr.To(enabled),
		},
	}

	obj, err := sdk.ToUnstructured(newCfg)
	if err != nil {
		return fmt.Errorf("failed to convert the '%s' module config: %v", name, err)
	}

	if _, err = kubeClient.Dynamic().Resource(v1alpha1.ModuleConfigGVR).Create(context.TODO(), obj, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create the '%s' module config: %v", name, err)
	}
	return nil
}
