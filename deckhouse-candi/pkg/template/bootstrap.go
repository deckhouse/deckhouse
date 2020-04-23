package template

import (
	"path/filepath"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
)

const bootstrapDir = "/bootstrap"

func PrepareBootstrap(templateController *Controller, nodeIP, bundleName string, metaConfig *config.MetaConfig) error {
	bashibleData := metaConfig.MarshalConfigForBashibleBundleTemplate(bundleName, nodeIP)

	saveInfo := []saveFromTo{
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
			from: filepath.Join(candiDir, "cloud-providers", metaConfig.ProviderName, "common-steps"),
			to:   bootstrapDir,
			data: bashibleData,
		},
	}

	for _, info := range saveInfo {
		logboek.LogInfoF("Rendering bootstrap templates from %q to %q\n", info.from, info.to)
		if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data); err != nil {
			return err
		}
	}

	return nil
}
