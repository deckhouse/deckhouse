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
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
)

var (
	candiDir         = "/deckhouse/candi"
	candiBashibleDir = candiDir + "/bashible"
)

const (
	bashibleDir = "/var/lib/bashible"
	stepsDir    = bashibleDir + "/bundle_steps"
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

func PrepareBundle(
	ctx context.Context,
	templateController *Controller,
	nodeIP string,
	devicePath string,
	metaConfig *config.MetaConfig,
	dc *directoryconfig.DirectoryConfig,
) error {
	bashibleData, err := metaConfig.ConfigForBashibleBundleTemplate(nodeIP)
	if err != nil {
		return err
	}
	logTemplatesData("bashible", bashibleData)

	if err := PrepareBashibleBundle(ctx, templateController, bashibleData, metaConfig.ProviderName, devicePath, dc); err != nil {
		return err
	}

	_, err = os.Stat(candiBashibleDir)
	if err != nil {
		if dc == nil {
			return fmt.Errorf("could not get value of dc.DownloadDir")
		}
		candiBashibleDir = filepath.Join(dc.DownloadDir, "deckhouse", "candi", "bashible")
	}

	bashboosterDir := filepath.Join(candiBashibleDir, "bashbooster")
	log.DebugF("From %q to %q\n", bashboosterDir, bashibleDir)
	return templateController.RenderBashBooster(bashboosterDir, bashibleDir, bashibleData)
}

//nolint:prealloc
func PrepareBashibleBundle(
	ctx context.Context,
	templateController *Controller,
	templateData map[string]interface{},
	provider string,
	devicePath string,
	dc *directoryconfig.DirectoryConfig,
) error {
	_, err := os.Stat(candiBashibleDir)
	if err != nil {
		if dc == nil {
			return fmt.Errorf("could not get value of dc.DownloadDir")
		}
		candiDir = filepath.Join(dc.DownloadDir, "deckhouse", "candi")
		candiBashibleDir = filepath.Join(dc.DownloadDir, "deckhouse", "candi", "bashible")
	}
	saveInfo := make([]saveFromTo, 0)
	saveInfo = append(saveInfo, saveFromTo{
		from: candiBashibleDir,
		to:   bashibleDir,
		data: templateData,
		ignorePaths: map[string]struct{}{
			filepath.Join(candiBashibleDir, "bootstrap.sh.tpl"): {},
		},
	})

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

// PreparePKI generates the control-plane PKI bundle and kubeconfig files
// inside templateController.TmpDir.
//
// controlPlaneEndpoint is the address that will be added to the apiserver
// certificate SAN list and used in kubeconfigs as the API server URL.
func PreparePKI(templateController *Controller, nodeName, nodeIP, controlPlaneEndpoint string, templateData map[string]interface{}) error {
	if templateController == nil {
		return fmt.Errorf("templateController is nil")
	}
	artifactsDir := filepath.Join(templateController.TmpDir+bashibleDir, "control-plane")
	return generatePKIArtifacts(nodeName, nodeIP, controlPlaneEndpoint, templateData, artifactsDir)
}

// generatePKIArtifacts writes PKI and kubeconfigs for the local
// control-plane node into artifactsDir. The function is decoupled from the
// template Controller for testability.
func generatePKIArtifacts(nodeName, nodeIP, controlPlaneEndpoint string, templateData map[string]interface{}, artifactsDir string) error {
	if nodeName == "" {
		return fmt.Errorf("nodeName is empty")
	}
	if controlPlaneEndpoint == "" {
		return fmt.Errorf("controlPlaneEndpoint is empty")
	}
	if artifactsDir == "" {
		return fmt.Errorf("artifactsDir is empty")
	}

	ip := net.ParseIP(nodeIP)
	if ip == nil {
		return fmt.Errorf("invalid node IP %q", nodeIP)
	}

	clusterCfg, ok := templateData["clusterConfiguration"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("templateData.clusterConfiguration is missing or has invalid type")
	}
	serviceCIDR, ok := clusterCfg["serviceSubnetCIDR"].(string)
	if !ok || serviceCIDR == "" {
		return fmt.Errorf("clusterConfiguration.serviceSubnetCIDR is missing or empty")
	}
	dnsDomain, ok := clusterCfg["clusterDomain"].(string)
	if !ok || dnsDomain == "" {
		return fmt.Errorf("clusterConfiguration.clusterDomain is missing or empty")
	}

	pkiDir := filepath.Join(artifactsDir, "pki")

	if _, err := pki.CreatePKIBundle(nodeName, dnsDomain, ip, serviceCIDR,
		pki.WithControlPlaneEndpoint(controlPlaneEndpoint),
		pki.WithPKIDir(pkiDir),
	); err != nil {
		return fmt.Errorf("create PKI bundle: %w", err)
	}

	kubeconfigFiles := []kubeconfig.File{
		kubeconfig.Kubelet,
		kubeconfig.Admin,
		kubeconfig.ControllerManager,
		kubeconfig.Scheduler,
		kubeconfig.SuperAdmin,
	}

	if _, err := kubeconfig.CreateKubeconfigFiles(kubeconfigFiles,
		kubeconfig.WithLocalAPIEndpoint(nodeIP),
		kubeconfig.WithNodeName(nodeName),
		kubeconfig.WithOutDir(filepath.Join(artifactsDir, "kubeconfig")),
		kubeconfig.WithCertificatesDir(pkiDir),
	); err != nil {
		return fmt.Errorf("create kubeconfig files: %w", err)
	}

	return nil
}

func PrepareControlPlaneManifests(templateController *Controller, templateData map[string]interface{}, dc *directoryconfig.DirectoryConfig) error {
	_, err := os.Stat(candiDir)
	if err != nil {
		if dc == nil {
			return fmt.Errorf("could not get value of dc.DownloadDir")
		}
		candiDir = filepath.Join(dc.DownloadDir, "deckhouse", "candi")
	}

	saveInfo := saveFromTo{
		from: filepath.Join(candiDir, "control-plane"),
		to:   filepath.Join(bashibleDir, "control-plane"),
		data: templateData,
	}
	log.InfoF("From %q to %q\n", saveInfo.from, saveInfo.to)
	if err := templateController.RenderAndSaveTemplates(saveInfo.from, saveInfo.to, saveInfo.data, nil); err != nil {
		return err
	}
	return nil
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
