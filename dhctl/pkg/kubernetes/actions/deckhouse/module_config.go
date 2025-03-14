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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func createModuleConfigManifestTask(ctx context.Context, kubeCl *client.KubernetesClient, mc *config.ModuleConfig, createMsg string) actions.ManifestTask {
	mcUnstructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mc)
	if err != nil {
		panic(err)
	}
	mcUnstruct := &unstructured.Unstructured{Object: mcUnstructMap}
	return actions.ManifestTask{
		Name: fmt.Sprintf(`ModuleConfig "%s"`, mc.GetName()),
		Manifest: func() interface{} {
			return mcUnstruct
		},
		CreateFunc: func(manifest interface{}) error {
			if createMsg != "" {
				log.InfoLn(createMsg)
			}
			// fake client does not support cache
			if _, ok := os.LookupEnv("DHCTL_TEST"); !ok {
				// need for invalidate cache
				_, err := kubeCl.APIResource(config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind)
				if err != nil {
					log.DebugF("Error getting mc api resource: %v\n", err)
				}
			}

			_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).
				Create(ctx, manifest.(*unstructured.Unstructured), metav1.CreateOptions{})
			if err != nil {
				log.DebugF("Do not create mc: %v\n", err)
			}

			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			// fake client does not support cache
			if _, ok := os.LookupEnv("DHCTL_TEST"); !ok {
				// need for invalidate cache
				_, err := kubeCl.APIResource(config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind)
				if err != nil {
					log.DebugF("Error getting mc api resource: %v\n", err)
				}
			}

			newManifest := manifest.(*unstructured.Unstructured)

			oldManifest, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Get(ctx, newManifest.GetName(), metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				log.DebugF("Error getting mc: %v\n", err)
			} else {
				newManifest.SetResourceVersion(oldManifest.GetResourceVersion())
			}

			_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).
				Update(ctx, newManifest, metav1.UpdateOptions{})
			if err != nil {
				log.InfoF("Do not updating mc: %v\n", err)
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

func setSettingToModuleConfig(ctx context.Context, kubeCl *client.KubernetesClient, mcName string, value interface{}, field []string) error {
	log.DebugF("setSettingToModuleConfig for mc %s, field %v, value %v", mcName, field, value)

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

	log.DebugLn("Found deckhouse mc. Try to prepare...")

	releaseChannel := ""
	releaseChannelRaw, hasReleaseChannelKey := mc.Spec.Settings["releaseChannel"]
	if rc, ok := releaseChannelRaw.(string); hasReleaseChannelKey && ok {
		log.DebugLn("Found releaseChannel in mc deckhouse. Remove it from mc")
		// we need set releaseChannel after bootstrapping process done
		// to prevent update during bootstrap
		delete(mc.Spec.Settings, "releaseChannel")
		releaseChannel = rc
	}

	if releaseChannel == "" {
		log.DebugLn("Not found releaseChannel in mc deckhouse. Finish preparing")
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

	log.DebugLn("Found global mc. Try to prepare...")

	var httpsSettings map[string]interface{}

	modulesRaw, hasModules := mc.Spec.Settings["modules"]
	if !hasModules {
		log.DebugLn("Not found modules in global mc. Finish preparing")
		return
	}

	modules, ok := modulesRaw.(map[string]interface{})
	if !ok {
		log.ErrorLn("modules is not map in global mc. Finish preparing")
		return
	}

	httpsRaw, hasHttps := modules["https"]
	if !hasHttps {
		log.DebugLn("Not found https in global mc. Finish preparing")
		return
	}

	httpsSettings, ok = httpsRaw.(map[string]interface{})
	if !ok {
		log.ErrorLn("https is not map in global mc. Finish preparing")
		return
	}

	if httpsSettings == nil {
		log.DebugLn("Not found httpsSettings in mc deckhouse. Finish preparing")
		return
	}

	log.DebugLn("Found https in global mc deckhouse. Remove it from mc")
	delete(modules, "https")
	if len(modules) == 0 {
		log.DebugLn("modules in global mc is empty. Remove it from mc")
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
