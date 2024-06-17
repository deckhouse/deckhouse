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

	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

const (
	candiDir         = "/deckhouse/candi"
	bashibleDir      = "/var/lib/bashible"
	candiBashibleDir = candiDir + "/bashible"
	stepsDir         = bashibleDir + "/bundle_steps"
	detectBundlePath = candiBashibleDir + "/detect_bundle.sh"
)

type saveFromTo struct {
	from        string
	to          string
	data        map[string]interface{}
	ignorePaths map[string]struct{}
}

func logTemplatesData(name string, data map[string]interface{}) {
	dataForLog := make(map[string]interface{})
	for k, v := range data {
		switch k {
		case "k8s", "bashible", "images":
			// Hide fields from the version map
			dataForLog[k] = "<hidden>"
		default:
			dataForLog[k] = v
		}
	}

	formattedData, _ := yaml.Marshal(dataForLog)

	log.DebugF("Data %s\n%s", name, string(formattedData))
}

func PrepareBundle(templateController *Controller, nodeIP, bundleName, devicePath string, metaConfig *config.MetaConfig) error {
	kubeadmData, err := metaConfig.ConfigForKubeadmTemplates("")
	if err != nil {
		return err
	}
	logTemplatesData("kubeadm", kubeadmData)

	bashibleData, err := metaConfig.ConfigForBashibleBundleTemplate(bundleName, nodeIP)
	if err != nil {
		return err
	}
	logTemplatesData("bashible", bashibleData)

	if err := PrepareBashibleBundle(templateController, bashibleData, metaConfig.ProviderName, bundleName, devicePath); err != nil {
		return err
	}

	if err := PrepareKubeadmConfig(templateController, kubeadmData); err != nil {
		return err
	}

	bashboosterDir := filepath.Join(candiBashibleDir, "bashbooster")
	log.DebugF("From %q to %q\n", bashboosterDir, bashibleDir)
	return templateController.RenderBashBooster(bashboosterDir, bashibleDir)
}

func PrepareBashibleBundle(templateController *Controller, templateData map[string]interface{}, provider, bundle, devicePath string) error {
	dataWithoutNodeGroup := withoutNodeGroup(templateData)
	getDataForStep := func(step string) map[string]interface{} {
		if step != "node-group" {
			return dataWithoutNodeGroup
		}
		return templateData
	}

	saveInfo := []saveFromTo{
		{
			from: candiBashibleDir,
			to:   bashibleDir,
			data: templateData,
			ignorePaths: map[string]struct{}{
				filepath.Join(candiBashibleDir, "bootstrap.sh.tpl"): {},
			},
		},
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "common-steps", steps),
			to:   stepsDir,
			data: getDataForStep(steps),
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "bundles", bundle, steps),
			to:   stepsDir,
			data: getDataForStep(steps),
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap", "node-group"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiDir, "cloud-providers", provider, "bashible", "common-steps", steps),
			to:   stepsDir,
			data: getDataForStep(steps),
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiDir, "cloud-providers", provider, "bashible", "bundles", bundle, steps),
			to:   stepsDir,
			data: dataWithoutNodeGroup,
		})
	}

	for _, info := range saveInfo {
		log.DebugF("From %q to %q\n", info.from, info.to)
		if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data, info.ignorePaths); err != nil {
			return err
		}
	}

	firstRunFileFlag := filepath.Join(templateController.TmpDir, bashibleDir, "first_run")
	log.DebugF("Create %q\n", firstRunFileFlag)
	if err := fs.CreateEmptyFile(firstRunFileFlag); err != nil {
		return err
	}

	devicePathFile := filepath.Join(templateController.TmpDir, bashibleDir, "kubernetes_data_device_path")
	log.InfoF("Create %q\n", devicePathFile)

	return fs.CreateFileWithContent(devicePathFile, devicePath)
}

func PrepareKubeadmConfig(templateController *Controller, templateData map[string]interface{}) error {
	saveInfo := []saveFromTo{
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm"),
			to:   filepath.Join(bashibleDir, "kubeadm"),
			data: templateData,
		},
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm", "patches"),
			to:   filepath.Join(bashibleDir, "kubeadm", "patches"),
			data: templateData,
		},
	}
	for _, info := range saveInfo {
		log.InfoF("From %q to %q\n", info.from, info.to)
		if err := templateController.RenderAndSaveTemplates(info.from, info.to, info.data, nil); err != nil {
			return err
		}
	}
	return nil
}

func withoutNodeGroup(data map[string]interface{}) map[string]interface{} {
	filteredData := make(map[string]interface{}, len(data))
	for key, value := range data {
		if key != "nodeGroup" {
			filteredData[key] = value
		}
	}
	return filteredData
}

func RenderAndSaveDetectBundle(data map[string]interface{}) (string, error) {
	log.DebugLn("Start render detect bundle script")

	return RenderAndSaveTemplate("detect_bundle.sh", detectBundlePath, data)
}
