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

package bootstrap

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
)

func (b *ClusterBootstrapper) CreateResources(ctx context.Context) error {
	resourcesToCreate := make(template.Resources, 0)
	if b.Options.Bootstrap.ResourcesPath != "" {
		log.WarnLn("--resources flag is deprecated. Please use --config flag multiple repeatedly for logical resources separation")
		parsedResources, err := template.ParseResources(b.Options.Bootstrap.ResourcesPath, nil)
		if err != nil {
			return err
		}

		resourcesToCreate = parsedResources
	} else {
		paths := fs.RevealWildcardPaths(b.Options.Global.ConfigPaths)
		for _, cfg := range paths {
			ress, err := template.ParseResources(cfg, nil)
			if err != nil {
				return err
			}

			resourcesToCreate = append(resourcesToCreate, ress...)
		}
	}

	log.DebugF("Resources: %s\n", resourcesToCreate.String())

	if len(resourcesToCreate) == 0 {
		log.WarnLn("Resources to create were not found.")
		return nil
	}

	interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
	if interactive {
		intLogger, ok := b.logger.(*log.InteractiveLogger)
		if !ok {
			return fmt.Errorf("logger is not interactive")
		}
		labelChan := intLogger.GetPhaseChan()
		phasesChan := make(chan phases.Progress, 5)
		pbParam := progressbar.NewPbParams(100, "Create resources", labelChan, phasesChan)

		if err := progressbar.InitProgressBar(pbParam); err != nil {
			return err
		}

		onComplete := func() {
			pb := progressbar.GetDefaultPb()
			pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
			pb.MultiPrinter.Stop()
		}
		defer onComplete()
	}

	return log.ProcessCtx(ctx, "bootstrap", "Create resources", func(ctx context.Context) error {
		kubeCl, err := b.KubeProvider.Client(ctx)
		if err != nil {
			return err
		}

		checkers, err := resources.GetCheckers(&client.KubernetesClient{KubeClient: kubeCl}, resourcesToCreate, nil)
		if err != nil {
			return err
		}

		return resources.CreateResourcesLoop(ctx, &client.KubernetesClient{KubeClient: kubeCl}, resourcesToCreate, checkers, nil, b.Options.Bootstrap.ResourcesTimeout)
	})
}
