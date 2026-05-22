// Copyright 2026 Flant JSC
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

package template

import (
	"context"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

const bootstrapDir = "/bootstrap"

func PrepareBootstrap(
	ctx context.Context,
	templateController *Controller,
	nodeIP string,
	metaConfig *config.MetaConfig,
	globalOptions *options.GlobalOptions,
) error {
	ctx, span := telemetry.StartSpan(ctx, "PrepareBootstrap")
	defer span.End()

	bashibleData, err := metaConfig.ConfigForBashibleBundleTemplate(nodeIP)
	if err != nil {
		return err
	}

	candiBashibleDir := filepath.Join(globalOptions.CandiDir, "bashible")

	saveInfo := []saveFromTo{
		{
			from: filepath.Join(candiBashibleDir, "bootstrap"),
			to:   bootstrapDir,
			data: bashibleData,
			ignorePaths: map[string]struct{}{
				filepath.Join(candiBashibleDir, "bootstrap", "02-bootstrap-bashible.sh.tpl"): {}, // will running in next stage
			},
		},
		{
			from: filepath.Join(globalOptions.CandiDir, "cloud-providers", metaConfig.ProviderName, "bashible", "common-steps"),
			to:   bootstrapDir,
			data: bashibleData,
		},
	}

	return log.ProcessCtx(ctx, "default", "Render bootstrap templates", func(ctx context.Context) error {
		for _, info := range saveInfo {
			log.InfoF("From %q to %q\n", info.from, info.to)
			if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data, info.ignorePaths); err != nil {
				return err
			}
		}

		return nil
	})
}
