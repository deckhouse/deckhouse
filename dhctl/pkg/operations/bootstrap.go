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

// TODO structure these functions into classes and move to the operations/bootstrap module
// TODO move states saving to operations/bootstrap/state.go

package operations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	ManifestCreatedInClusterCacheKey = "tf-state-and-manifests-in-cluster"
	MasterHostsCacheKey              = "cluster-hosts"
	BastionHostCacheKey              = "bastion-hosts"
)

func BootstrapMaster(sshClient *ssh.Client, bundleName, nodeIP string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return log.Process("bootstrap", "Initial bootstrap", func() error {
		if err := template.PrepareBootstrap(controller, nodeIP, bundleName, metaConfig); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)
			err := log.Process("default", bootstrapScript, func() error {
				if _, err := os.Stat(scriptPath); err != nil {
					if os.IsNotExist(err) {
						log.InfoF("Script %s wasn't found\n", scriptPath)
						return nil
					}
					return fmt.Errorf("script path: %v", err)
				}
				cmd := sshClient.UploadScript(scriptPath).
					WithStdoutHandler(func(l string) { log.InfoLn(l) }).
					Sudo()

				_, err := cmd.Execute()
				if err != nil {
					return fmt.Errorf("run %s: %w", scriptPath, err)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func PrepareBashibleBundle(bundleName, nodeIP, devicePath string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return log.Process("bootstrap", "Prepare Bashible Bundle", func() error {
		return template.PrepareBundle(controller, nodeIP, bundleName, devicePath, metaConfig)
	})
}

func ExecuteBashibleBundle(sshClient *ssh.Client, tmpDir string) error {
	return log.Process("bootstrap", "Execute Bashible Bundle", func() error {
		bundleCmd := sshClient.UploadScript("bashible.sh", "--local").Sudo()
		parentDir := tmpDir + "/var/lib"
		bundleDir := "bashible"

		_, err := bundleCmd.ExecuteBundle(parentDir, bundleDir)
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("bundle '%s' error: %v\nstderr: %s", bundleDir, err, string(ee.Stderr))
			}
			return fmt.Errorf("bundle '%s' error: %v", bundleDir, err)
		}
		return nil
	})
}

const (
	bashibleInstalledMessage = `Bashible is already installed and healthy!
	%s
`
	bashibleIsNotReadyMessage = `Bashible is not ready! Let's try to install it ...
	Reason: %s
`
)

func CheckBashibleBundle(sshClient *ssh.Client) bool {
	var bashibleUpToDate bool
	_ = log.Process("bootstrap", "Check Bashible", func() error {
		bashibleCmd := sshClient.Command("/var/lib/bashible/bashible.sh", "--local").
			Sudo().WithTimeout(20 * time.Second)
		var output string

		err := bashibleCmd.WithStdoutHandler(func(l string) {
			if output == "" {
				output = l
			} else {
				return
			}
			switch {
			case strings.Contains(output, "Can't acquire lockfile /var/lock/bashible."):
				fallthrough
			case strings.Contains(output, "Configuration is in sync, nothing to do."):
				log.InfoF(bashibleInstalledMessage, output)
				bashibleUpToDate = true
			default:
				log.InfoF(bashibleIsNotReadyMessage, output)
			}
		}).Run()
		if err != nil {
			log.DebugLn(err.Error())
		}
		return nil
	})
	return bashibleUpToDate
}

func RunBashiblePipeline(sshClient *ssh.Client, cfg *config.MetaConfig, nodeIP, devicePath string) error {
	bundleName, err := DetermineBundleName(sshClient)
	if err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	_ = log.Process("bootstrap", "Rendered templates directory", func() error {
		log.InfoLn(templateController.TmpDir)
		return nil
	})

	if err := BootstrapMaster(sshClient, bundleName, nodeIP, cfg, templateController); err != nil {
		return err
	}

	if ok := CheckBashibleBundle(sshClient); ok {
		return nil
	}

	if err = PrepareBashibleBundle(bundleName, nodeIP, devicePath, cfg, templateController); err != nil {
		return err
	}
	tomb.RegisterOnShutdown("Delete templates temporary directory", func() {
		if !app.IsDebug {
			_ = os.RemoveAll(templateController.TmpDir)
		}
	})

	if err := ExecuteBashibleBundle(sshClient, templateController.TmpDir); err != nil {
		return err
	}

	return RebootMaster(sshClient)
}

func DetermineBundleName(sshClient *ssh.Client) (string, error) {
	var bundleName string
	err := log.Process("bootstrap", "Detect Bashible Bundle", func() error {
		file, err := template.RenderAndSaveDetectBundle(make(map[string]interface{}))
		if err != nil {
			return err
		}

		return retry.NewSilentLoop("Get bundle", 3, 1*time.Second).Run(func() error {
			// run detect bundle type
			detectCmd := sshClient.UploadScript(file)
			stdout, err := detectCmd.Execute()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return fmt.Errorf("detect_bundle.sh: %v, %s", err, string(ee.Stderr))
				}
				return fmt.Errorf("detect_bundle.sh: %v", err)
			}

			bundleName = strings.Trim(string(stdout), "\n ")
			if bundleName == "" {
				return fmt.Errorf("detect_bundle.sh: empty bundle was detected")
			}

			log.InfoF("Detected bundle: %s\n", bundleName)
			return nil
		})
	})
	return bundleName, err
}

func WaitForSSHConnectionOnMaster(sshClient *ssh.Client) error {
	return log.Process("bootstrap", "Wait for SSH on Master become Ready", func() error {
		availabilityCheck := sshClient.Check()
		_ = log.Process("default", "Connection string", func() error {
			log.InfoLn(availabilityCheck.String())
			return nil
		})
		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(); err != nil {
			return fmt.Errorf("await master to become available: %v", err)
		}
		return nil
	})
}

func InstallDeckhouse(kubeCl *client.KubernetesClient, config *deckhouse.Config) error {
	return log.Process("bootstrap", "Install Deckhouse", func() error {
		err := bootstrap.CheckPreventBreakAnotherBootstrappedCluster(kubeCl, config)
		if err != nil {
			return err
		}

		err = deckhouse.CreateDeckhouseManifests(kubeCl, config)
		if err != nil {
			return fmt.Errorf("deckhouse create manifests: %v", err)
		}

		err = cache.Global().Save(ManifestCreatedInClusterCacheKey, []byte("yes"))
		if err != nil {
			return fmt.Errorf("set manifests in cluster flag to cache: %v", err)
		}

		err = deckhouse.WaitForReadiness(kubeCl)
		if err != nil {
			return fmt.Errorf("deckhouse install: %v", err)
		}

		return nil
	})
}

func ConnectToKubernetesAPI(sshClient *ssh.Client) (*client.KubernetesClient, error) {
	var kubeCl *client.KubernetesClient
	err := log.Process("common", "Connect to Kubernetes API", func() error {
		if sshClient != nil {
			if err := sshClient.Check().WithDelaySeconds(1).AwaitAvailability(); err != nil {
				return fmt.Errorf("await master available: %v", err)
			}
		}

		err := retry.NewLoop("Get Kubernetes API client", 45, 5*time.Second).Run(func() error {
			kubeCl = client.NewKubernetesClient()
			if sshClient != nil {
				kubeCl = kubeCl.WithSSHClient(sshClient)
			}
			if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		time.Sleep(50 * time.Millisecond) // tick to prevent first probable fail
		err = deckhouse.WaitForKubernetesAPI(kubeCl)
		if err != nil {
			return fmt.Errorf("wait kubernetes api: %v", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("start kubernetes proxy: %v", err)
	}

	return kubeCl, nil
}

// Different Linux distributions may have different return codes. In most debian and centos based it is 255, in altlinux and possibly in some others it is 1.
const rebootExitCode = 255
const alternativeRebootExitCode = 1

func RebootMaster(sshClient *ssh.Client) error {
	return log.Process("bootstrap", "Reboot Master️", func() error {
		rebootCmd := sshClient.Command("sudo", "reboot").Sudo().
			WithSSHArgs("-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2")
		if err := rebootCmd.Run(); err != nil {
			ee, ok := err.(*exec.ExitError)
			if ok {
				if ee.ExitCode() == rebootExitCode || ee.ExitCode() == alternativeRebootExitCode {
					return nil
				}
			}
			return fmt.Errorf("shutdown error: exit_code: %v stdout: %s stderr: %s %v",
				ee.ExitCode(),
				rebootCmd.StdoutBuffer.String(),
				rebootCmd.StderrBuffer.String(),
				err,
			)
		}
		log.InfoLn("OK!")
		return nil
	})
}

func BootstrapTerraNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraNodeGroups []config.TerraNodeGroupSpec) error {
	for _, ng := range terraNodeGroups {
		err := log.Process("bootstrap", fmt.Sprintf("Create %s NodeGroup", ng.Name), func() error {
			err := converge.CreateNodeGroup(kubeCl, ng.Name, metaConfig.NodeGroupManifest(ng))
			if err != nil {
				return err
			}

			cloudConfig, err := converge.GetCloudConfig(kubeCl, ng.Name, converge.ShowDeckhouseLogs)
			if err != nil {
				return err
			}

			for i := 0; i < ng.Replicas; i++ {
				err = converge.BootstrapAdditionalNode(kubeCl, metaConfig, i, "static-node", ng.Name, cloudConfig, false)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func SaveMasterHostsToCache(hosts map[string]string) {
	if err := cache.Global().SaveStruct(MasterHostsCacheKey, hosts); err != nil {
		log.DebugF("Cannot save ssh hosts %v", err)
	}
}

func GetMasterHostsIPs() ([]string, error) {
	var hosts map[string]string
	err := cache.Global().LoadStruct(MasterHostsCacheKey, &hosts)
	if err != nil {
		return nil, err
	}
	mastersIPs := make([]string, 0, len(hosts))
	for _, ip := range hosts {
		mastersIPs = append(mastersIPs, ip)
	}

	sort.Strings(mastersIPs)

	return mastersIPs, nil
}

func SaveBastionHostToCache(host string) {
	if err := cache.Global().Save(BastionHostCacheKey, []byte(host)); err != nil {
		log.ErrorF("Cannot save ssh hosts: %v\n", err)
	}
}

func GetBastionHostFromCache() (string, error) {
	exists, err := cache.Global().InCache(BastionHostCacheKey)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", nil
	}

	host, err := cache.Global().Load(BastionHostCacheKey)
	if err != nil {
		return "", err
	}

	return string(host), nil
}

func BootstrapAdditionalMasterNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, addressTracker map[string]string) error {
	if metaConfig.MasterNodeGroupSpec.Replicas == 1 {
		log.DebugF("Skip bootstrap additional master nodes because replicas == 1")
		return nil
	}

	return log.Process("bootstrap", "Bootstrap additional master nodes", func() error {
		masterCloudConfig, err := converge.GetCloudConfig(kubeCl, converge.MasterNodeGroupName, converge.ShowDeckhouseLogs)
		if err != nil {
			return err
		}

		for i := 1; i < metaConfig.MasterNodeGroupSpec.Replicas; i++ {
			outputs, err := converge.BootstrapAdditionalMasterNode(kubeCl, metaConfig, i, masterCloudConfig, false)
			if err != nil {
				return err
			}
			addressTracker[fmt.Sprintf("%s-master-%d", metaConfig.ClusterPrefix, i)] = outputs.MasterIPForSSH

			SaveMasterHostsToCache(addressTracker)
		}

		return nil
	})
}

func BootstrapGetNodesFromCache(metaConfig *config.MetaConfig, stateCache state.Cache) (map[string]map[int]string, error) {
	nodeGroupRegex := fmt.Sprintf("^%s-(.*)-([0-9]+)\\.tfstate$", metaConfig.ClusterPrefix)
	groupsReg, _ := regexp.Compile(nodeGroupRegex)

	nodesFromCache := make(map[string]map[int]string)

	err := stateCache.Iterate(func(name string, content []byte) error {
		switch {
		case strings.HasSuffix(name, ".backup"):
			fallthrough
		case strings.HasPrefix(name, "base-infrastructure"):
			fallthrough
		case strings.HasPrefix(name, "uuid"):
			fallthrough
		case !groupsReg.MatchString(name):
			return nil
		}

		nodeGroupNameAndNodeIndex := groupsReg.FindStringSubmatch(name)

		nodeGroupName := nodeGroupNameAndNodeIndex[1]
		rawIndex := nodeGroupNameAndNodeIndex[2]

		index, convErr := strconv.Atoi(rawIndex)
		if convErr != nil {
			return fmt.Errorf("can't convert %q to integer: %v", rawIndex, convErr)
		}

		if _, ok := nodesFromCache[nodeGroupName]; !ok {
			nodesFromCache[nodeGroupName] = make(map[int]string)
		}

		nodesFromCache[nodeGroupName][index] = strings.TrimSuffix(name, ".tfstate")
		return nil
	})
	return nodesFromCache, err
}
