package template

import (
	"flant/deckhouse-candi/pkg/log"
	"fmt"
	"path/filepath"

	"github.com/flant/logboek"
	"gopkg.in/yaml.v2"

	"flant/deckhouse-candi/pkg/config"
)

const (
	candiDir         = "/deckhouse/candi"
	bashibleDir      = "/var/lib/bashible"
	candiBashibleDir = candiDir + "/bashible"
	stepsDir         = bashibleDir + "/bundle_steps"
)

type saveFromTo struct {
	from string
	to   string
	data map[string]interface{}
}

func logTemplatesData(name string, data map[string]interface{}) {
	formattedData, _ := yaml.Marshal(data)
	_ = logboek.LogProcess(fmt.Sprintf("%s data", name), log.BoldOptions(), func() error {
		logboek.LogInfoF(string(formattedData))
		return nil
	})
}

func PrepareBundle(templateController *Controller, nodeIP, bundleName string, metaConfig *config.MetaConfig) error {
	kubeadmData := metaConfig.MarshalConfigForKubeadmTemplates(nodeIP)
	logTemplatesData("kubeadm", kubeadmData)

	bashibleData := metaConfig.MarshalConfigForBashibleBundleTemplate(bundleName, nodeIP)
	logTemplatesData("bashible", bashibleData)

	return logboek.LogProcess("Render bashible bundle templates", log.BoldOptions(), func() error {
		if err := PrepareBashibleBundle(templateController, bashibleData, metaConfig.ProviderName, bundleName); err != nil {
			return err
		}

		if err := PrepareKubeadmConfig(templateController, kubeadmData); err != nil {
			return err
		}

		bashboosterDir := filepath.Join(candiBashibleDir, "bashbooster")
		logboek.LogInfoF("From %q to %q\n", bashboosterDir, bashibleDir)
		return templateController.RenderBashBooster(bashboosterDir, bashibleDir)
	})
}

func PrepareBashibleBundle(templateController *Controller, templateData map[string]interface{}, provider, bundle string) error {
	saveInfo := []saveFromTo{
		{
			from: candiBashibleDir,
			to:   bashibleDir,
			data: templateData,
		},
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "common-steps", steps),
			to:   stepsDir,
			data: templateData,
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "bundles", bundle, steps),
			to:   stepsDir,
			data: templateData,
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiDir, "cloud-providers", provider, "bashible", "bundles", bundle, steps),
			to:   stepsDir,
			data: templateData,
		})
	}
	for _, info := range saveInfo {
		logboek.LogInfoF("From %q to %q\n", info.from, info.to)
		if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data); err != nil {
			return err
		}
	}
	return nil
}

func PrepareKubeadmConfig(templateController *Controller, templateData map[string]interface{}) error {
	saveInfo := []saveFromTo{
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm"),
			to:   filepath.Join(bashibleDir, "kubeadm"),
			data: templateData,
		},
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm", "kustomize"),
			to:   filepath.Join(bashibleDir, "kubeadm", "kustomize"),
			data: templateData,
		},
	}
	for _, info := range saveInfo {
		logboek.LogInfoF("From %q to %q\n", info.from, info.to)
		if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data); err != nil {
			return err
		}
	}
	return nil
}
