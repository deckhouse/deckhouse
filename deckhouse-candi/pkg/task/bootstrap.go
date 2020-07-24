package task

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/template"
)

func BootstrapMaster(sshClient *ssh.SshClient, bundleName, nodeIP string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return logboek.LogProcess("🔨 Run Master Bootstrap 🔨", log.TaskOptions(), func() error {
		if err := template.PrepareBootstrap(controller, nodeIP, bundleName, metaConfig); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
			scriptPath := controller.TmpDir + "/bootstrap/" + bootstrapScript
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

func PrepareBashibleBundle(bundleName, nodeIP string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return logboek.LogProcess("📦 Prepare Bashible Bundle 📦", log.TaskOptions(), func() error {
		return template.PrepareBundle(controller, nodeIP, bundleName, metaConfig)
	})
}

func ExecuteBashibleBundle(sshClient *ssh.SshClient, tmpDir string) error {
	return logboek.LogProcess("🚁 Run Bashible Bundle 🚁", log.TaskOptions(), func() error {
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
	err := logboek.LogProcess("🐛 Detect Bashible Bundle 🐛", log.TaskOptions(), func() error {
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
	return logboek.LogProcess("🐾 Wait for SSH on master become ready 🐾", log.TaskOptions(), func() error {
		err := sshClient.Check().AwaitAvailability()
		if err != nil {
			return fmt.Errorf("await master available: %v", err)
		}
		return nil
	})
}

func InstallDeckhouse(kubeCl *kube.KubernetesClient, config *deckhouse.Config, nodeGroupConfig map[string]interface{}) error {
	return logboek.LogProcess("🍀 Install Deckhouse 🍀", log.TaskOptions(), func() error {
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

		err = deckhouse.CreateNodeGroup(kubeCl, "master", nodeGroupConfig)
		if err != nil {
			return err
		}

		return nil
	})
}

func StartKubernetesAPIProxy(sshClient *ssh.SshClient) (*kube.KubernetesClient, error) {
	var kubeCl *kube.KubernetesClient
	err := logboek.LogProcess("🕸️ Start Kubernetes API proxy 🕸️", log.TaskOptions(), func() error {
		kubeCl = kube.NewKubernetesClient().WithSshClient(sshClient)
		if err := kubeCl.Init(""); err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("start kubernetes proxy: %v", err)
	}
	return kubeCl, nil
}

const rebootExitCode = 255

func RebootMaster(sshClient *ssh.SshClient) error {
	return logboek.LogProcess("🌪️ Reboot master 🌪️", log.TaskOptions(), func() error {
		rebootCmd := sshClient.Command("sudo", "reboot").Sudo().WithSSHArgs("-o", "ServerAliveInterval=15")
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

func BootstrapStaticNodes(kubeCl *kube.KubernetesClient, metaConfig *config.MetaConfig, staticNodeGroups []config.StaticNodeGroupSpec) error {
	for _, staticNodeGroup := range staticNodeGroups {
		err := deckhouse.CreateNodeGroup(kubeCl, staticNodeGroup.Name, metaConfig.MarshalNodeGroupConfig(staticNodeGroup))
		if err != nil {
			return err
		}

		nodeCloudConfig, err := deckhouse.GetCloudConfig(kubeCl, staticNodeGroup.Name)
		if err != nil {
			return err
		}

		for i := 0; i < staticNodeGroup.Replicas; i++ {
			err = deckhouse.BootstrapAdditionalNode(kubeCl, i, metaConfig.ProviderName, metaConfig.Layout, "static-node", staticNodeGroup.Name, nodeCloudConfig, metaConfig)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func BootstrapAdditionalMasterNodes(kubeCl *kube.KubernetesClient, metaConfig *config.MetaConfig, replicas int) error {
	masterCloudConfig, err := deckhouse.GetCloudConfig(kubeCl, "master")
	if err != nil {
		return err
	}

	for i := 1; i < replicas; i++ {
		err = deckhouse.BootstrapAdditionalNode(kubeCl, i, metaConfig.ProviderName, metaConfig.Layout, "master-node", "master", masterCloudConfig, metaConfig)
		if err != nil {
			return err
		}
	}
	return nil
}
