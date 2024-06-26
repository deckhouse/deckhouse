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

// TODO structure these functions into classes
// TODO move states saving to operations/bootstrap/state.go

package bootstrap

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net/http"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
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

		err := log.Process("bootstrap", fmt.Sprintf("Prepare %s", app.NodeDeckhouseDirectoryPath), func() error {
			if err := sshClient.Command("mkdir", "-p", "-m", "0755", app.NodeDeckhouseDirectoryPath).Sudo().Run(); err != nil {
				return fmt.Errorf("ssh: mkdir -p -m 0755 %s: %w", app.NodeDeckhouseDirectoryPath, err)
			}
			if err := sshClient.Command("mkdir", "-p", app.DeckhouseNodeBinPath).Sudo().Run(); err != nil {
				return fmt.Errorf("ssh: mkdir -p %s: %w", app.DeckhouseNodeBinPath, err)
			}
			if err := sshClient.Command("mkdir", "-p", "-m", "1777", app.DeckhouseNodeTmpPath).Sudo().Run(); err != nil {
				return fmt.Errorf("ssh: mkdir -p -m 1777 %s: %w", app.DeckhouseNodeTmpPath, err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot create %s directories: %w", app.NodeDeckhouseDirectoryPath, err)
		}

		for _, bootstrapScript := range []string{"01-base-pkgs.sh", "02-network-scripts.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)
			err := log.Process("default", bootstrapScript, func() error {
				if _, err := os.Stat(scriptPath); err != nil {
					if os.IsNotExist(err) {
						log.InfoF("Script %s wasn't found\n", scriptPath)
						return nil
					}
					return fmt.Errorf("script path: %v", err)
				}
				logs := make([]string, 0)
				cmd := sshClient.UploadScript(scriptPath).
					WithStdoutHandler(func(l string) {
						logs = append(logs, l)
						log.DebugLn(l)
					}).Sudo()

				_, err := cmd.Execute()
				if err != nil {
					log.ErrorLn(strings.Join(logs, "\n"))
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
			var ee *exec.ExitError
			if errors.As(err, &ee) {
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

func SetupSSHTunnelToRegistryPackagesProxy(sshCl *ssh.Client) (*frontend.ReverseTunnel, error) {
	tun := sshCl.ReverseTunnel("5444:127.0.0.1:5444")
	err := tun.Up()
	if err != nil {
		return nil, err
	}

	return tun, nil
}

type registryClientConfigGetter struct {
	registry.ClientConfig
}

func newRegistryClientConfigGetter(config config.RegistryData) (*registryClientConfigGetter, error) {
	auth, err := config.Auth()
	if err != nil {
		return nil, fmt.Errorf("registry auth: %v", err)
	}

	repo := fmt.Sprintf("%s/%s", strings.Trim(config.Address, "/"), strings.Trim(config.Path, "/"))

	return &registryClientConfigGetter{
		ClientConfig: registry.ClientConfig{
			Repository: repo,
			Scheme:     config.Scheme,
			CA:         config.CA,
			Auth:       auth,
		},
	}, nil
}

func (r *registryClientConfigGetter) Get(_ string) (*registry.ClientConfig, error) {
	return &r.ClientConfig, nil
}

func StartRegistryPackagesProxy(config config.RegistryData, clusterDomain string) error {
	cert, err := generateTLSCertificate(clusterDomain)
	if err != nil {
		return fmt.Errorf("Failed to generate TLS certificate for registry proxy: %v", err)
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:5444", &tls.Config{
		Certificates: []tls.Certificate{*cert},
	})
	if err != nil {
		return fmt.Errorf("Failed to listen registry proxy socket: %v", err)
	}

	clientConfigGetter, err := newRegistryClientConfigGetter(config)
	if err != nil {
		return fmt.Errorf("Failed to create registry client for registry proxy: %v", err)
	}

	proxy := proxy.NewProxy(&http.Server{}, listener, clientConfigGetter, registryPackagesProxyLogger{}, &registry.DefaultClient{})

	go proxy.Serve()

	return nil
}

type registryPackagesProxyLogger struct{}

func (r registryPackagesProxyLogger) Errorf(format string, args ...interface{}) {
	log.ErrorF(format, args...)
}

func (r registryPackagesProxyLogger) Infof(format string, args ...interface{}) {
	log.InfoF(format, args...)
}

func (r registryPackagesProxyLogger) Warnf(format string, args ...interface{}) {
	log.WarnF(format, args...)
}

func (r registryPackagesProxyLogger) Debugf(format string, args ...interface{}) {
	log.DebugF(format, args...)
}

func (r registryPackagesProxyLogger) Error(args ...interface{}) {
	log.ErrorLn(args...)
}

func generateTLSCertificate(clusterDomain string) (*tls.Certificate, error) {
	now := time.Now()

	subjectKeyId := make([]byte, 10)

	_, err := rand.Read(subjectKeyId)
	if err != nil {
		return nil, fmt.Errorf("failed to generate subject key id: %v", err)
	}

	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName:         fmt.Sprintf("registry-packages-proxy.%s", clusterDomain),
			Country:            []string{"Unknown"},
			Organization:       []string{clusterDomain},
			OrganizationalUnit: []string{"registry-packages-proxy"},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, 1), // Valid for one day
		SubjectKeyId:          subjectKeyId,
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate,
		priv.Public(), priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  priv,
	}

	return tlsCert, nil
}

func RunBashiblePipeline(sshClient *ssh.Client, cfg *config.MetaConfig, nodeIP, devicePath string) error {
	if err := CheckDHCTLDependencies(sshClient); err != nil {
		return err
	}

	bundleName, err := DetermineBundleName(sshClient)
	if err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	log.DebugF("Rendered templates directory %s\n", templateController.TmpDir)

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

	return ExecuteBashibleBundle(sshClient, templateController.TmpDir)
}

const dependencyCmd = "type"

func CheckDHCTLDependencies(sshClient *ssh.Client) error {
	return log.Process("bootstrap", "Check DHCTL Dependencies", func() error {
		dependencyArgs := []string{"sudo", "rm", "tar", "mount", "awk", "grep", "cut", "sed", "shopt",
			"mkdir", "cp", "join"}

		for _, args := range dependencyArgs {
			log.InfoF("Check dependency %s\n", args)
			output, err := sshClient.Command(dependencyCmd, args).CombinedOutput()
			if err != nil {
				return fmt.Errorf("bashible dependency error: %s",
					string(output),
				)
			}
		}
		log.InfoLn("OK!")
		return nil
	})
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
				var ee *exec.ExitError
				if errors.As(err, &ee) {
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

func InstallDeckhouse(kubeCl *client.KubernetesClient, config *config.DeckhouseInstaller) error {
	return log.Process("bootstrap", "Install Deckhouse", func() error {
		err := CheckPreventBreakAnotherBootstrappedCluster(kubeCl, config)
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

const rebootExitCode = 255

func RebootMaster(sshClient *ssh.Client) error {
	return log.Process("bootstrap", "Reboot Master️", func() error {
		rebootCmd := sshClient.Command("reboot").Sudo().
			WithSSHArgs("-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2")
		if err := rebootCmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				if ee.ExitCode() == rebootExitCode {
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

func BootstrapTerraNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraNodeGroups []config.TerraNodeGroupSpec, terraformContext *terraform.TerraformContext) error {
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
				err = converge.BootstrapAdditionalNode(kubeCl, metaConfig, i, "static-node", ng.Name, cloudConfig, false, terraformContext)
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

func BootstrapAdditionalMasterNodes(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, addressTracker map[string]string, terraformContext *terraform.TerraformContext) error {
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
			outputs, err := converge.BootstrapAdditionalMasterNode(kubeCl, metaConfig, i, masterCloudConfig, false, terraformContext)
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
