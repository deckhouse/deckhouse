// Copyright 2022 Flant JSC
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

package registry

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8env "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DefineRegistryCommand(kpApp *kingpin.Application) {
	repositoryCmd := kpApp.Command("registry", "Deckhouse repository work.").
		PreAction(func(_ *kingpin.ParseContext) error {
			kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
			return nil
		})

	repositoryListCmd := repositoryCmd.Command("list", "List in registry").
		PreAction(func(_ *kingpin.ParseContext) error {
			kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
			return nil
		})

	repositoryListCmd.Command("releases", "Show releases list.")

	moduleSource := repositoryListCmd.Flag("source", "Module source name.").String()
	moduleName := repositoryListCmd.Flag("name", "Module name.").String()
	moduleChannel := repositoryListCmd.Flag("channel", "Module channel").ExistingFile()

	repositoryListCmd.Action(func(_ *kingpin.ParseContext) error {
		ctx := context.Background()

		fmt.Println("listing releases")
		dc := dependency.NewDependencyContainer()

		scheme := runtime.NewScheme()
		utilruntime.Must(v1alpha1.AddToScheme(scheme))

		restConfig := ctrl.GetConfigOrDie()
		k8sClient, err := client.New(restConfig, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		// get relevant module source
		ms := new(v1alpha1.ModuleSource)
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: *moduleSource}, ms); err != nil {
			return fmt.Errorf("get ModuleSource %s got an error: %w", moduleSource, err)
		}

		svc := NewService(dc, d8env.GetDownloadedModulesDir(), ms, utils.GenerateRegistryOptions(ms))
		if err := svc.DownloadMetadataFromReleaseChannel(*moduleName, *moduleChannel, ""); err != nil {
			fmt.Println("error", err)
		}

		return nil
	})
}
