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
	"errors"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/sdk"
	kubeclient "github.com/flant/kube-client/client"
	"gopkg.in/alecthomas/kingpin.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
			cli := kubeclient.New(kubeclient.WithLogger(logger))
			if err := cli.Init(); err != nil {
				return fmt.Errorf("init: %w", err)
			}

			return moduleSwitch(cli, moduleName, true, "enable", logger)
		})
	moduleEnableCmd.Arg("module_name", "").Required().StringVar(&moduleName)

	moduleDisableCmd := moduleCmd.Command("disable", "Disable module via spec.enabled flag in the ModuleConfig resource. Use snake-case for the module name.").
		Action(func(_ *kingpin.ParseContext) error {
			logger.SetLevel(log.LevelError)
			cli := kubeclient.New(kubeclient.WithLogger(logger))
			if err := cli.Init(); err != nil {
				return fmt.Errorf("init: %w", err)
			}

			return moduleSwitch(cli, moduleName, false, "disable", logger)
		})
	moduleDisableCmd.Arg("module_name", "").Required().StringVar(&moduleName)
}

func moduleSwitch(kubeClient *kubeclient.Client, moduleName string, enabled bool, actionDesc string, logger *log.Logger) error {
	// TODO: check formatters?
	// log.SetFormatter(&log.TextFormatter{DisableTimestamp: true, ForceColors: true})
	logger.SetLevel(log.LevelError)

	if err := setModuleConfigEnabled(context.TODO(), kubeClient, moduleName, enabled); err != nil {
		return fmt.Errorf("%s module: %w", actionDesc, err)
	}

	fmt.Printf("Module %s %sd\n", moduleName, actionDesc)
	return nil
}

// setModuleConfigEnabled updates spec.enabled flag or creates a new ModuleConfig with spec.enabled flag.
func setModuleConfigEnabled(ctx context.Context, kubeClient k8s.Client, name string, enabled bool) error {
	// this should not happen, but check it anyway.
	if kubeClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	if enabled {
		unstructuredObjModule, err := kubeClient.Dynamic().Resource(v1alpha1.ModuleGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return errors.New("module not found")
			}
			return fmt.Errorf("get the '%s' module: %w", name, err)
		}

		sources, ok, _ := unstructured.NestedStringSlice(unstructuredObjModule.Object, "properties", "availableSources")
		source, _, _ := unstructured.NestedString(unstructuredObjModule.Object, "properties", "source")
		if ok && len(sources) > 1 && source == "" {
			fmt.Printf("Warning: module '%s' is enabled but didn’t run because multiple sources were found (%s), please specify a source in ModuleConfig resource\n", name, strings.Join(sources, ", "))
		}
	}

	unstructuredObj, err := kubeClient.Dynamic().Resource(v1alpha1.ModuleConfigGVR).Get(ctx, name, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("get the '%s' module config: %w", name, err)
	}

	if unstructuredObj != nil {
		if err = unstructured.SetNestedField(unstructuredObj.Object, enabled, "spec", "enabled"); err != nil {
			return fmt.Errorf("change spec.enabled to %v in the '%s' module config: %w", enabled, name, err)
		}
		if _, err = kubeClient.Dynamic().Resource(v1alpha1.ModuleConfigGVR).Update(ctx, unstructuredObj, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update the '%s' module config: %w", name, err)
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
		return fmt.Errorf("convert the '%s' module config: %w", name, err)
	}

	if _, err = kubeClient.Dynamic().Resource(v1alpha1.ModuleConfigGVR).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create the '%s' module config: %w", name, err)
	}

	return nil
}
