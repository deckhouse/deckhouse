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
	return log.BootstrapProcess("Initial bootstrap", func() error {
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
	return log.BootstrapProcess("Prepare Bashible Bundle", func() error {
		return template.PrepareBundle(controller, nodeIP, bundleName, devicePath, metaConfig)
	})
}

func ExecuteBashibleBundle(sshClient *ssh.SshClient, tmpDir string) error {
	return log.BootstrapProcess("Execute Bashible Bundle", func() error {
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

func RunBashiblePipeline(sshClient *ssh.SshClient, cfg *config.MetaConfig, nodeIP, devicePath string) error {
	bundleName, err := DetermineBundleName(sshClient)
	if err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	_ = log.BoldProcess("Rendered templates directory", func() error {
		logboek.LogInfoLn(templateController.TmpDir)
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

func DetermineBundleName(sshClient *ssh.SshClient) (string, error) {
	var bundleName string
	err := log.BootstrapProcess("Detect Bashible Bundle", func() error {
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
	return log.BootstrapProcess("Wait for SSH on Master become Ready", func() error {
		availabilityCheck := sshClient.Check()
		_ = log.BoldProcess("Connection string", func() error {
			logboek.LogInfoLn(availabilityCheck.String())
			return nil
		})
		if err := availabilityCheck.AwaitAvailability(); err != nil {
			return fmt.Errorf("await master available: %v", err)
		}
		return nil
	})
}

func InstallDeckhouse(kubeCl *client.KubernetesClient, config *deckhouse.Config, nodeGroupConfig map[string]interface{}) error {
	return log.BootstrapProcess("Install Deckhouse", func() error {
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
	err := log.CommonProcess("Start Kubernetes API proxy", func() error {
		if err := sshClient.Check().AwaitAvailability(); err != nil {
			return fmt.Errorf("await master available: %v", err)
		}
		return retry.StartLoop("Kubernetes API proxy", 45, 20, func() error {
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
	return log.BootstrapProcess("Reboot Master️", func() error {
		rebootCmd := sshClient.Command("sudo", "reboot").Sudo().WithSSHArgs("-o", "ServerAliveCountMax=2")
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
		logboek.LogInfoLn("OK!")
		return nil
	})
}

func BootstrapStaticNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, staticNodeGroups []config.StaticNodeGroupSpec) error {
	for _, staticNodeGroup := range staticNodeGroups {
		err := log.BootstrapProcess(fmt.Sprintf("Create %s NodeGroup", staticNodeGroup.Name), func() error {
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
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func BootstrapAdditionalMasterNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, replicas int) error {
	return log.BootstrapProcess("Create master NodeGroup", func() error {
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
	})
}
