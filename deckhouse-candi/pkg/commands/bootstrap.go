package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/converge"
	"flant/deckhouse-candi/pkg/kubernetes/actions/deckhouse"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
	"flant/deckhouse-candi/pkg/template"
	"flant/deckhouse-candi/pkg/util/retry"
)

func BootstrapMaster(sshClient *ssh.SshClient, bundleName, nodeIP string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return logboek.LogProcess("🛠️ ~ Run Master Bootstrap", log.TaskOptions(), func() error {
		if err := template.PrepareBootstrap(controller, nodeIP, bundleName, metaConfig); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)
			err := logboek.LogProcess("Run "+bootstrapScript, log.BoldOptions(), func() error {
				if _, err := os.Stat(scriptPath); err != nil {
					if os.IsNotExist(err) {
						logboek.LogInfoF("Script %s doesn't found\n", scriptPath)
						return nil
					}
					return fmt.Errorf("script path: %v", err)
				}
				cmd := sshClient.UploadScript(scriptPath).
					WithStdoutHandler(func(l string) { logboek.LogInfoLn(l) }).
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
	return logboek.LogProcess("📦 ~ Prepare Bashible Bundle", log.TaskOptions(), func() error {
		return template.PrepareBundle(controller, nodeIP, bundleName, devicePath, metaConfig)
	})
}

func ExecuteBashibleBundle(sshClient *ssh.SshClient, tmpDir string) error {
	return logboek.LogProcess("🚁 ~ Execute Bashible Bundle", log.TaskOptions(), func() error {
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

func DetermineBundleName(sshClient *ssh.SshClient) (string, error) {
	var bundleName string
	err := logboek.LogProcess("🔍 ~ Detect Bashible Bundle", log.TaskOptions(), func() error {
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
		logboek.LogInfoF("Detected bundle: %s\n", bundleName)

		return nil
	})
	return bundleName, err
}

func WaitForSSHConnectionOnMaster(sshClient *ssh.SshClient) error {
	return logboek.LogProcess("🚥 ~ Wait for SSH on Master become ready", log.TaskOptions(), func() error {
		availabilityCheck := sshClient.Check()
		logboek.LogInfoF("Verifying connection: %q\n\n", availabilityCheck.String())
		if err := availabilityCheck.AwaitAvailability(); err != nil {
			return fmt.Errorf("await master available: %v", err)
		}
		return nil
	})
}

func InstallDeckhouse(kubeCl *client.KubernetesClient, config *deckhouse.Config, nodeGroupConfig map[string]interface{}) error {
	return logboek.LogProcess("🐳 ~ Install Deckhouse", log.TaskOptions(), func() error {
		err := deckhouse.WaitForKubernetesAPI(kubeCl)
		if err != nil {
			return fmt.Errorf("deckhouse wait api: %v", err)
		}

		err = deckhouse.CreateDeckhouseManifests(kubeCl, config)
		if err != nil {
			return fmt.Errorf("deckhouse create manifests: %v", err)
		}

		err = deckhouse.WaitForReadiness(kubeCl, config)
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

func StartKubernetesAPIProxy(sshClient *ssh.SshClient) (*client.KubernetesClient, error) {
	var kubeCl *client.KubernetesClient
	err := logboek.LogProcess("🚤 ~ Start Kubernetes API proxy", log.TaskOptions(), func() error {
		return retry.StartLoop("Waiting Kubernetes API proxy", 45, 20, func() error {
			kubeCl = client.NewKubernetesClient().WithSSHClient(sshClient)
			if err := kubeCl.Init(""); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("start kubernetes proxy: %v", err)
	}
	return kubeCl, nil
}

const rebootExitCode = 255

func RebootMaster(sshClient *ssh.SshClient) error {
	return logboek.LogProcess("⛺ ~ Reboot Master️", log.TaskOptions(), func() error {
		rebootCmd := sshClient.Command("sudo", "reboot").Sudo().WithSSHArgs("-o", "ServerAliveCountMax=3")
		if err := rebootCmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				if ee.ExitCode() == rebootExitCode {
					return nil
				}
			}
			return fmt.Errorf("shutdown error: stdout: %s stderr: %s %v", rebootCmd.StdoutBuffer.String(), rebootCmd.StderrBuffer.String(), err)
		}
		return nil
	})
}

func BootstrapStaticNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, staticNodeGroups []config.StaticNodeGroupSpec) error {
	for _, staticNodeGroup := range staticNodeGroups {
		err := converge.CreateNodeGroup(kubeCl, staticNodeGroup.Name, metaConfig.MarshalNodeGroupConfig(staticNodeGroup))
		if err != nil {
			return err
		}

		nodeCloudConfig, err := converge.GetCloudConfig(kubeCl, staticNodeGroup.Name)
		if err != nil {
			return err
		}

		for i := 0; i < staticNodeGroup.Replicas; i++ {
			err = converge.BootstrapAdditionalNode(kubeCl, i, metaConfig.ProviderName, metaConfig.Layout, "static-node", staticNodeGroup.Name, nodeCloudConfig, metaConfig)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func BootstrapAdditionalMasterNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, replicas int) error {
	masterCloudConfig, err := converge.GetCloudConfig(kubeCl, "master")
	if err != nil {
		return err
	}

	for i := 1; i < replicas; i++ {
		err = converge.BootstrapAdditionalMasterNode(kubeCl, i, metaConfig.ProviderName, metaConfig.Layout, masterCloudConfig, metaConfig)
		if err != nil {
			return err
		}
	}
	return nil
}
