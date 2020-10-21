package operations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/converge"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/template"
	"flant/candictl/pkg/util/cache"
	"flant/candictl/pkg/util/retry"
)

func BootstrapMaster(sshClient *ssh.SSHClient, bundleName, nodeIP string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return log.Process("bootstrap", "Initial bootstrap", func() error {
		if err := template.PrepareBootstrap(controller, nodeIP, bundleName, metaConfig); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)
			err := log.Process("default", bootstrapScript, func() error {
				if _, err := os.Stat(scriptPath); err != nil {
					if os.IsNotExist(err) {
						log.InfoF("Script %s doesn't found\n", scriptPath)
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

func ExecuteBashibleBundle(sshClient *ssh.SSHClient, tmpDir string) error {
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

func RunBashiblePipeline(sshClient *ssh.SSHClient, cfg *config.MetaConfig, nodeIP, devicePath string) error {
	bundleName, err := DetermineBundleName(sshClient)
	if err != nil {
		return err
	}

	var bashibleUpToDate bool
	_ = log.Process("bootstrap", "Check Bashible", func() error {
		bashibleCmd := sshClient.Command("bash", "/var/lib/bashible/bashible.sh", "--local").
			Cmd().Sudo().WithTimeout(3 * time.Second)
		var output string

		err = bashibleCmd.WithStdoutHandler(func(l string) { output += l + "\n" }).Run()
		if err != nil {
			log.DebugF("%v\n", err)
		}

		output = strings.TrimSuffix(output, "\n")
		switch {
		case strings.Contains(output, "Can't acquire lockfile /var/lock/bashible."):
			fallthrough
		case strings.Contains(output, "Configuration is in sync, nothing to do."):
			log.InfoF(bashibleInstalledMessage, output)
			bashibleUpToDate = true
		default:
			log.InfoF(bashibleIsNotReadyMessage, output)
		}
		return nil
	})
	if bashibleUpToDate {
		return nil
	}

	templateController := template.NewTemplateController("")
	_ = log.Process("bootstrap", "Rendered templates directory", func() error {
		log.InfoLn(templateController.TmpDir)
		return nil
	})

	if err := BootstrapMaster(sshClient, bundleName, nodeIP, cfg, templateController); err != nil {
		return err
	}
	if err = PrepareBashibleBundle(bundleName, nodeIP, devicePath, cfg, templateController); err != nil {
		return err
	}
	if err := ExecuteBashibleBundle(sshClient, templateController.TmpDir); err != nil {
		return err
	}
	if err := RebootMaster(sshClient); err != nil {
		return err
	}
	return nil
}

func DetermineBundleName(sshClient *ssh.SSHClient) (string, error) {
	var bundleName string
	err := log.Process("bootstrap", "Detect Bashible Bundle", func() error {
		// run detect bundle type
		detectCmd := sshClient.UploadScript("/deckhouse/candi/bashible/detect_bundle.sh")
		stdout, err := detectCmd.Execute()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("script '%s' error: %v\nstderr: %s", "detect_bundle.sh", err, string(ee.Stderr))
			}
			return fmt.Errorf("script '%s' error: %v", "detect_bundle.sh", err)
		}

		bundleName = strings.Trim(string(stdout), "\n ")
		log.InfoF("Detected bundle: %s\n", bundleName)

		return nil
	})
	return bundleName, err
}

func WaitForSSHConnectionOnMaster(sshClient *ssh.SSHClient) error {
	return log.Process("bootstrap", "Wait for SSH on Master become Ready", func() error {
		availabilityCheck := sshClient.Check()
		_ = log.Process("default", "Connection string", func() error {
			log.InfoLn(availabilityCheck.String())
			return nil
		})
		if err := availabilityCheck.WithDelaySeconds(3).AwaitAvailability(); err != nil {
			return fmt.Errorf("await master to become available: %v", err)
		}
		return nil
	})
}

func InstallDeckhouse(kubeCl *client.KubernetesClient, config *deckhouse.Config, nodeGroupConfig map[string]interface{}) error {
	return log.Process("bootstrap", "Install Deckhouse", func() error {
		err := deckhouse.CreateDeckhouseManifests(kubeCl, config)
		if err != nil {
			return fmt.Errorf("deckhouse create manifests: %v", err)
		}

		err = deckhouse.WaitForReadiness(kubeCl)
		if err != nil {
			return fmt.Errorf("deckhouse install: %v", err)
		}

		err = converge.CreateNodeGroup(kubeCl, "master", nodeGroupConfig)
		if err != nil {
			return err
		}

		return nil
	})
}

func StartKubernetesAPIProxy(sshClient *ssh.SSHClient) (*client.KubernetesClient, error) {
	var kubeCl *client.KubernetesClient
	err := log.Process("common", "Start Kubernetes API proxy", func() error {
		if err := sshClient.Check().WithDelaySeconds(3).AwaitAvailability(); err != nil {
			return fmt.Errorf("await master available: %v", err)
		}
		err := retry.StartLoop("Kubernetes API proxy", 45, 5, func() error {
			kubeCl = client.NewKubernetesClient().WithSSHClient(sshClient)
			if err := kubeCl.Init(""); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		<-time.After(time.Second) // tick to prevent first probable fail

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

const rebootExitCode = 255

func RebootMaster(sshClient *ssh.SSHClient) error {
	return log.Process("bootstrap", "Reboot Master️", func() error {
		rebootCmd := sshClient.Command("sudo", "reboot").Sudo().
			WithSSHArgs("-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2")
		if err := rebootCmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				if ee.ExitCode() == rebootExitCode {
					return nil
				}
			}
			return fmt.Errorf("shutdown error: stdout: %s stderr: %s %v",
				rebootCmd.StdoutBuffer.String(),
				rebootCmd.StderrBuffer.String(),
				err,
			)
		}
		log.InfoLn("OK!")
		return nil
	})
}

func BootstrapStaticNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, staticNodeGroups []config.StaticNodeGroupSpec) error {
	for _, ng := range staticNodeGroups {
		err := log.Process("bootstrap", fmt.Sprintf("Create %s NodeGroup", ng.Name), func() error {
			err := converge.CreateNodeGroup(kubeCl, ng.Name, metaConfig.NodeGroupManifest(ng))
			if err != nil {
				return err
			}

			cloudConfig, err := converge.GetCloudConfig(kubeCl, ng.Name)
			if err != nil {
				return err
			}

			for i := 0; i < ng.Replicas; i++ {
				err = converge.BootstrapAdditionalNode(kubeCl, metaConfig, i, "static-node", ng.Name, cloudConfig)
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

func BootstrapAdditionalMasterNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, addressTracker map[string]string, replicas int) error {
	return log.Process("bootstrap", "Create master NodeGroup", func() error {
		masterCloudConfig, err := converge.GetCloudConfig(kubeCl, "master")
		if err != nil {
			return err
		}

		for i := 1; i < replicas; i++ {
			outputs, err := converge.BootstrapAdditionalMasterNode(kubeCl, metaConfig, i, masterCloudConfig)
			if err != nil {
				return err
			}
			addressTracker[fmt.Sprintf("%s-master-%d", metaConfig.ClusterPrefix, i)] = outputs.MasterIPForSSH
		}

		return nil
	})
}

func BootstrapGetNodesFromCache(metaConfig *config.MetaConfig, stateCache cache.Cache) (map[string]map[int]string, error) {
	nodeGroupRegex := fmt.Sprintf("^%s-(.*)-([0-9]+)$", metaConfig.ClusterPrefix)
	groupsReg, _ := regexp.Compile(nodeGroupRegex)

	nodesFromCache := make(map[string]map[int]string)
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || strings.HasSuffix(path, ".backup") {
			return nil
		}

		if strings.HasPrefix(info.Name(), "base-infrastructure") || strings.HasPrefix(info.Name(), "uuid") {
			return nil
		}

		name := strings.TrimSuffix(info.Name(), ".tfstate")
		if !groupsReg.MatchString(name) {
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

		nodesFromCache[nodeGroupName][index] = name
		return nil
	}

	if err := filepath.Walk(stateCache.GetDir(), walkFunc); err != nil {
		return nil, fmt.Errorf("can't iterate the cache: %v", err)
	}

	return nodesFromCache, nil
}
