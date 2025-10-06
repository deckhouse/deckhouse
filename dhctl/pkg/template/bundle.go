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

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

var (
	candiDir         = "/deckhouse/candi"
	candiBashibleDir = candiDir + "/bashible"
)

const (
	bashibleDir = "/var/lib/bashible"
	stepsDir    = bashibleDir + "/bundle_steps"
)

const (
	kubeadmV1Beta4MinKubeVersion = "1.31.0"
	kubeadmV1Beta4               = "v1beta4"
	kubeadmV1Beta3               = "v1beta3"
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

func PrepareBundle(templateController *Controller, nodeIP, devicePath string, metaConfig *config.MetaConfig) error {
	kubeadmData, err := metaConfig.ConfigForKubeadmTemplates("")
	if err != nil {
		return err
	}
	logTemplatesData("kubeadm", kubeadmData)

	bashibleData, err := metaConfig.ConfigForBashibleBundleTemplate(nodeIP)
	if err != nil {
		return err
	}
	logTemplatesData("bashible", bashibleData)

	if err := PrepareBashibleBundle(templateController, bashibleData, metaConfig.ProviderName, devicePath); err != nil {
		return err
	}

	if err := PrepareKubeadmConfig(templateController, kubeadmData); err != nil {
		return err
	}

	bashboosterDir := filepath.Join(candiBashibleDir, "bashbooster")
	log.DebugF("From %q to %q\n", bashboosterDir, bashibleDir)
	return templateController.RenderBashBooster(bashboosterDir, bashibleDir, bashibleData)
}

func PrepareBashibleBundle(templateController *Controller, templateData map[string]interface{}, provider, devicePath string) error {
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

	for _, steps := range []string{"all", "cluster-bootstrap"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiBashibleDir, "common-steps", steps),
			to:   stepsDir,
			data: templateData,
		})
	}

	for _, steps := range []string{"all", "cluster-bootstrap"} {
		saveInfo = append(saveInfo, saveFromTo{
			from: filepath.Join(candiDir, "cloud-providers", provider, "bashible", "common-steps", steps),
			to:   stepsDir,
			data: templateData,
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

func GetKubeadmVersion(kubernetesVersion string) (string, error) {
	v, err := semver.NewVersion(kubernetesVersion)
	if err != nil {
		return "", err
	}

	minConstraint, _ := semver.NewConstraint(">=" + kubeadmV1Beta4MinKubeVersion)

	if minConstraint.Check(v) {
		return kubeadmV1Beta4, nil
	}
	return kubeadmV1Beta3, nil
}

func PrepareKubeadmConfig(templateController *Controller, templateData map[string]interface{}) error {
	cc := templateData["clusterConfiguration"].(map[string]interface{})
	k8sVer := cc["kubernetesVersion"].(string)
	kubeadmVersion, err := GetKubeadmVersion(k8sVer)
	if err != nil {
		return err
	}

	saveInfo := []saveFromTo{
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm", kubeadmVersion),
			to:   filepath.Join(bashibleDir, "kubeadm", kubeadmVersion),
			data: templateData,
		},
		{
			from: filepath.Join(candiDir, "control-plane-kubeadm", kubeadmVersion, "patches"),
			to:   filepath.Join(bashibleDir, "kubeadm", kubeadmVersion, "patches"),
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

func InitGlobalVars(pwd string) {
	candiDir = pwd + "/deckhouse/candi"
	candiBashibleDir = candiDir + "/bashible"
	checkPortsScriptPath = candiBashibleDir + "/preflight/check_ports.sh.tpl"
	checkLocalhostScriptPath = candiBashibleDir + "/preflight/check_localhost.sh.tpl"
	checkDeckhouseUserScriptPath = candiBashibleDir + "/preflight/check_deckhouse_user.sh.tpl"
	preflightScriptDirPath = candiBashibleDir + "/preflight/"
	killReverseTunnelPath = candiBashibleDir + "/preflight/kill_reverse_tunnel.sh.tpl"
	checkProxyRevTunnelOpenScriptPath = candiBashibleDir + "/preflight/check_reverse_tunnel_open.sh.tpl"
}
