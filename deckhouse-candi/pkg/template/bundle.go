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
		logboek.LogInfoF("\n%s\n", string(formattedData))
		return nil
	})
}

func PrepareBundle(templateController *Controller, nodeIP, bundleName string, metaConfig *config.MetaConfig) error {
	kubeadmData := metaConfig.MarshalConfigForKubeadmTemplates(nodeIP)
	logTemplatesData("kubeadm", kubeadmData)

	bashibleData := metaConfig.MarshalConfigForBashibleBundleTemplate(bundleName, nodeIP)
	logTemplatesData("bashible", bashibleData)

	saveInfo := []saveFromTo{
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm"),
			to:   filepath.Join(bashibleDir, "kubeadm"),
			data: kubeadmData,
		},
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm", "kustomize"),
			to:   filepath.Join(bashibleDir, "kubeadm", "kustomize"),
			data: kubeadmData,
		},
		{
			from: candiBashibleDir,
			to:   bashibleDir,
			data: bashibleData,
		},
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "common-steps", steps),
			to:   stepsDir,
			data: bashibleData,
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "bundles", bundleName, steps),
			to:   stepsDir,
			data: bashibleData,
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiDir, "cloud-providers", metaConfig.ProviderName, "bashible", "bundles", bundleName, steps),
			to:   stepsDir,
			data: bashibleData,
		})
	}

	for _, info := range saveInfo {
		logboek.LogInfoF("Rendering bundle templates from %q to %q\n", info.from, info.to)
		if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data); err != nil {
			return err
		}
	}

	logboek.LogInfoF("Rendering bashbooster\n")
	if err := templateController.RenderBashBooster(filepath.Join(candiBashibleDir, "bashbooster"), bashibleDir); err != nil { //nolint:lll
		return err
	}
	return nil
}
