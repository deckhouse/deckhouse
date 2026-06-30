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

package deckhouse

import (
	"context"
	"fmt"
	"os"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func removeResourceVersion(mc *unstructured.Unstructured) {
	// try to multiple deleting
	mc.SetResourceVersion("")
	unstructured.RemoveNestedField(mc.Object, "metadata", "resourceVersion")
}

func createModuleConfigManifestTask(kubeCl *client.KubernetesClient, mc *config.ModuleConfig, createMsg string) actions.ManifestTask {
	mcUnstructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mc)
	if err != nil {
		panic(err)
	}

	mcUnstruct := &unstructured.Unstructured{Object: mcUnstructMap}
	removeResourceVersion(mcUnstruct)

	return actions.ManifestTask{
		Name: fmt.Sprintf(`ModuleConfig "%s"`, mc.GetName()),
		Manifest: func() any {
			return mcUnstruct
		},
		CreateFunc: func(ctx context.Context, manifest any) error {
			if createMsg != "" {
				dhlog.FromContext(ctx).InfoContext(ctx, createMsg)
			}

			// fake client does not support cache
			if _, ok := os.LookupEnv("DHCTL_TEST"); !ok {
				// need for invalidate cache
				_, err := kubeCl.APIResource(config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind)
				if err != nil {
					dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error getting mc api resource: %v", err))
				}
			}

			m := manifest.(*unstructured.Unstructured)

			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Resource version before delete field for mc %s: '%s'", m.GetResourceVersion(), m.GetName()))
			removeResourceVersion(m)
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Resource version after delete field for mc %s: '%s'", m.GetResourceVersion(), m.GetName()))

			_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Create(ctx, m, metav1.CreateOptions{})
			if err != nil {
				dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Not creating mc: %v", err))
			}

			return err
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			// fake client does not support cache
			if _, ok := os.LookupEnv("DHCTL_TEST"); !ok {
				// need for invalidate cache
				_, err := kubeCl.APIResource(config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind)
				if err != nil {
					dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error getting mc api resource: %v", err))
				}
			}

			newManifest := manifest.(*unstructured.Unstructured)

			oldManifest, err := kubeCl.
				Dynamic().Resource(config.ModuleConfigGVR).
				Get(ctx, newManifest.GetName(), metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Error getting mc: %v", err))
			} else {
				newManifest.SetResourceVersion(oldManifest.GetResourceVersion())
			}

			_, err = kubeCl.
				Dynamic().Resource(config.ModuleConfigGVR).
				Update(ctx, newManifest, metav1.UpdateOptions{})
			if err != nil {
				dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Not updating mc: %v", err))
			}

			return err
		},
	}
}

func prepareModuleConfig(ctx context.Context, mc *config.ModuleConfig, res *ManifestsResult) {
	// we need apply some settings after bootstrap and with creating resources
	// but not apply when we are creating module configs into cluster
	// see description for functions
	switch mc.GetName() {
	case "deckhouse":
		prepareDeckhouseMC(ctx, mc, res)
	case "global":
		prepareGlobalMC(ctx, mc, res)
	}
}

func setSettingToModuleConfig(ctx context.Context, kubeCl *client.KubernetesClient, mcName string, value any, field []string) error {
	dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("setSettingToModuleConfig for mc %s, field %v, value %v", mcName, field, value), "\n"))

	cm, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Get(ctx, mcName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	fieldPath := append([]string{"spec", "settings"}, field...)

	err = unstructured.SetNestedField(cm.Object, value, fieldPath...)
	if err != nil {
		return err
	}

	_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func prepareDeckhouseMC(ctx context.Context, mc *config.ModuleConfig, res *ManifestsResult) {
	// we should apply releaseChannel setting after bootstrap cluster
	// for preventing an updating deckhouse during bootstrap process
	// for example, we are installing v1.66 tag but in release channel we have v1.67 tag

	dhlog.FromContext(ctx).DebugContext(ctx, "Found deckhouse mc. Trying to prepare...")

	releaseChannel := ""
	releaseChannelRaw, hasReleaseChannelKey := mc.Spec.Settings["releaseChannel"]
	if rc, ok := releaseChannelRaw.(string); hasReleaseChannelKey && ok {
		dhlog.FromContext(ctx).DebugContext(ctx, "Found releaseChannel in mc deckhouse. Removing it from mc")
		// we need set releaseChannel after bootstrapping process done
		// to prevent update during bootstrap
		delete(mc.Spec.Settings, "releaseChannel")
		releaseChannel = rc
	}

	if releaseChannel == "" {
		dhlog.FromContext(ctx).DebugContext(ctx, "releaseChannel not found in mc deckhouse. Finished preparing")
		return
	}

	res.PostBootstrapMCTasks = append(res.PostBootstrapMCTasks, actions.ModuleConfigTask{
		Title: "Set release channel to deckhouse module config",
		Do: func(kubeCl *client.KubernetesClient) error {
			return setSettingToModuleConfig(ctx, kubeCl, "deckhouse", releaseChannel, []string{"releaseChannel"})
		},
		Name: "deckhouse",
	})
}

func prepareGlobalMC(ctx context.Context, mc *config.ModuleConfig, res *ManifestsResult) {
	// we should apply setting only after bootstrap cloud permanent node
	// imagine, we have https custom certificate setting
	// if we apply with this, we will have deckhouse in error state because secret will be created in resource
	// and deckhouse cannot found this secret and cloud permanent nodes will not bootstrap
	// because deckhouse stuck in error and it cannot create manual-for-bootstrap secrets

	dhlog.FromContext(ctx).DebugContext(ctx, "Found global mc. Trying to prepare...")

	var httpsSettings map[string]any

	modulesRaw, hasModules := mc.Spec.Settings["modules"]
	if !hasModules {
		dhlog.FromContext(ctx).DebugContext(ctx, "modules not found in global mc. Finished preparing")
		return
	}

	modules, ok := modulesRaw.(map[string]any)
	if !ok {
		dhlog.FromContext(ctx).ErrorContext(ctx, "modules is not a map in global mc. Finished preparing")
		return
	}

	HTTPSRaw, hasHTTPS := modules["https"]
	if !hasHTTPS {
		dhlog.FromContext(ctx).DebugContext(ctx, "https not found in global mc. Finished preparing")
		return
	}

	httpsSettings, ok = HTTPSRaw.(map[string]any)
	if !ok {
		dhlog.FromContext(ctx).ErrorContext(ctx, "https is not a map in global mc. Finished preparing")
		return
	}

	if httpsSettings == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, "httpsSettings not found in mc deckhouse. Finished preparing")
		return
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "Found https in global mc deckhouse. Removing it from mc")
	delete(modules, "https")
	if len(modules) == 0 {
		dhlog.FromContext(ctx).DebugContext(ctx, "modules in global mc is empty. Removing it from mc")
		delete(mc.Spec.Settings, "modules")
	}

	res.WithResourcesMCTasks = append(res.WithResourcesMCTasks, actions.ModuleConfigTask{
		Title: "Set https setting to global module config",
		Do: func(kubeCl *client.KubernetesClient) error {
			return setSettingToModuleConfig(ctx, kubeCl, "global", httpsSettings, []string{"modules", "https"})
		},
		Name: "global",
	})
}
