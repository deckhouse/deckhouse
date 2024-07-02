// Copyright 2021 Flant JSC
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
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const bootstrapDir = "/bootstrap"

func PrepareBootstrap(templateController *Controller, nodeIP, bundleName string, metaConfig *config.MetaConfig) error {
	bashibleData, err := metaConfig.ConfigForBashibleBundleTemplate(bundleName, nodeIP)
	if err != nil {
		return err
	}
	saveInfo := []saveFromTo{
		{
			from: filepath.Join(candiBashibleDir, "bootstrap"),
			to:   bootstrapDir,
			data: bashibleData,
			ignorePaths: map[string]struct{}{
				filepath.Join(candiBashibleDir, "bootstrap", "03-prepare-bashible.sh.tpl"): {}, // will running in next stage
			},
		},
		{
			from: filepath.Join(candiBashibleDir, "bundles", bundleName),
			to:   bootstrapDir,
			data: bashibleData,
		},
		{
			from: filepath.Join(candiDir, "cloud-providers", metaConfig.ProviderName, "bashible", "bundles", bundleName),
			to:   bootstrapDir,
			data: bashibleData,
		},
		{
			from: filepath.Join(candiDir, "cloud-providers", metaConfig.ProviderName, "bashible", "common-steps"),
			to:   bootstrapDir,
			data: bashibleData,
		},
	}

	return log.Process("default", "Render bootstrap templates", func() error {
		for _, info := range saveInfo {
			log.InfoF("From %q to %q\n", info.from, info.to)
			if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data, info.ignorePaths); err != nil {
				return err
			}
		}
		return nil
	})
}
