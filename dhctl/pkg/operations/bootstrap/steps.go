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
	"encoding/json"
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
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

const (
	ManifestCreatedInClusterCacheKey  = "tf-state-and-manifests-in-cluster"
	MasterHostsCacheKey               = "cluster-hosts"
	BastionHostCacheKey               = "bastion-hosts"
	DHCTLEndBootstrapBashiblePipeline = app.NodeDeckhouseDirectoryPath + "/first-control-plane-bashible-ran"
)

func BootstrapMaster(nodeInterface node.Interface, controller *template.Controller) error {
	return log.Process("bootstrap", "Initial bootstrap", func() error {
		for _, bootstrapScript := range []string{"01-network-scripts.sh", "02-base-pkgs.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)

			err := retry.NewLoop(fmt.Sprintf("Execute %s", bootstrapScript), 30, 5*time.Second).
				Run(func() error {
					if _, err := os.Stat(scriptPath); err != nil {
						if os.IsNotExist(err) {
							log.InfoF("Script %s wasn't found\n", scriptPath)
							return nil
						}
						return fmt.Errorf("script path: %v", err)
					}
					logs := make([]string, 0)
					cmd := nodeInterface.UploadScript(scriptPath)
					cmd.WithStdoutHandler(func(l string) {
						logs = append(logs, l)
						log.DebugLn(l)
					})
					cmd.Sudo()

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

func ExecuteBashibleBundle(nodeInterface node.Interface, tmpDir string) error {
	bundleCmd := nodeInterface.UploadScript("bashible.sh", "--local")
	bundleCmd.WithCleanupAfterExec(false)
	bundleCmd.Sudo()
	parentDir := tmpDir + "/var/lib"
	bundleDir := "bashible"

	_, err := bundleCmd.ExecuteBundle(parentDir, bundleDir)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("bundle '%s' error: %v\nstderr: %s", bundleDir, err, string(ee.Stderr))
		}

		if errors.Is(err, frontend.ErrBashibleTimeout) {
			return frontend.ErrBashibleTimeout
		}

		return fmt.Errorf("bundle '%s' error: %v", bundleDir, err)
	}
	return nil
}

func checkBashibleAlreadyRun(nodeInterface node.Interface) (bool, error) {
	isReady := false
	err := log.Process("bootstrap", "Checking bashible is ready", func() error {
		cmd := nodeInterface.Command("cat", DHCTLEndBootstrapBashiblePipeline)
		cmd.Sudo()
		cmd.WithTimeout(10 * time.Second)
		if err := cmd.Run(); err != nil {
			isReady = false
			return err
		}

		stdout := string(cmd.StdoutBytes())
		log.DebugF("cat %s stdout: '%s'\n", DHCTLEndBootstrapBashiblePipeline, stdout)

		isReady = strings.TrimSpace(stdout) == "OK"

		return nil
	})

	return isReady, err
}

func getBashiblePIDs(nodeInterface node.Interface) ([]string, error) {
	var psStrings []string
	h := func(l string) {
		psStrings = append(psStrings, l)
	}
	cmd := nodeInterface.Command("bash", "-c", `ps a --no-headers -o args:64 -o "|%p"`)
	cmd.Sudo()
	cmd.WithTimeout(10 * time.Second)
	cmd.WithStdoutHandler(h)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		// ssh exits with the exit status of the remote command or with 255 if an error occurred.
		if errors.As(err, &ee) {
			log.DebugF("'ps a --no-headers -o args:64 -o \"|%%p\"' got exit code: %d and stderr %s", ee.ExitCode(), string(ee.Stderr))
			if ee.ExitCode() == 255 {
				return nil, err
			}
		}

		return nil, err
	}

	var res []string
	for _, l := range psStrings {
		log.DebugF("ps string: '%s'\n", l)

		parts := strings.SplitN(l, "|", 2)
		if len(parts) < 2 {
			log.DebugLn("Skip ps string without pid")
			continue
		}

		if !strings.Contains(parts[0], "bashible.sh") {
			continue
		}

		pid := strings.TrimSpace(parts[1])
		log.DebugF("Found bashible PID: %s\n", pid)

		res = append(res, pid)
	}

	return res, nil
}

func killBashible(nodeInterface node.Interface, pids []string) error {
	cmd := nodeInterface.Command("kill", pids...)
	cmd.Sudo()
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		// ssh exits with the exit status of the remote command or with 255 if an error occurred.
		if errors.As(err, &ee) {
			log.DebugF("'kill %v' got exit code: %d and stderr %s", pids, ee.ExitCode(), string(ee.Stderr))
			if ee.ExitCode() == 255 {
				return err
			}

			return nil
		}
	}

	return nil
}

func unlockBashible(NodeInterface node.Interface) error {
	cmd := NodeInterface.Command("rm", "-f", "/var/lock/bashible")
	cmd.Sudo()
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func cleanupPreviousBashibleRunIfNeed(nodeInterface node.Interface) error {
	return log.Process("bootstrap", "Cleanup previous bashible run if need", func() error {
		log.DebugF("Gettting bashible pids")
		pids, err := getBashiblePIDs(nodeInterface)
		if err != nil {
			return err
		}

		log.DebugLn("Got bashible pids: %v", pids)
		if len(pids) == 0 {
			log.InfoLn("Bashible instance not found. Start it!")
			return nil
		}

		if err := killBashible(nodeInterface, pids); err != nil {
			return err
		}

		return unlockBashible(nodeInterface)
	})
}

func SetupSSHTunnelToRegistryPackagesProxy(sshCl *ssh.Client) (*frontend.ReverseTunnel, error) {
	port := "5444"
	listenAddress := "127.0.0.1"

	checkingScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(
		fmt.Sprintf("https://localhost:%s/healthz", port))
	if err != nil {
		return nil, fmt.Errorf("Cannot render reverse tunnel checking script: %v", err)
	}

	killScript, err := template.RenderAndSaveKillReverseTunnelScript(
		listenAddress, port)
	if err != nil {
		return nil, fmt.Errorf("Cannot render kill reverse tunnel script: %v", err)
	}

	checker := ssh.NewRunScriptReverseTunnelChecker(sshCl, checkingScript)
	killer := ssh.NewRunScriptReverseTunnelKiller(sshCl, killScript)

	tun := sshCl.ReverseTunnel(fmt.Sprintf("%s:%s:%s:%s", listenAddress, port, listenAddress, port))
	err = tun.Up()
	if err != nil {
		return nil, err
	}

	tun.StartHealthMonitor(checker, killer)

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
	srv := &http.Server{}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	proxy := proxy.NewProxy(srv, listener, clientConfigGetter, registryPackagesProxyLogger{}, &registry.DefaultClient{})

	go proxy.Serve()

	return nil
}

type registryPackagesProxyLogger struct{}

func (r registryPackagesProxyLogger) Errorf(format string, args ...interface{}) {
	log.ErrorF(format+"\n", args...)
}

func (r registryPackagesProxyLogger) Infof(format string, args ...interface{}) {
	log.InfoF(format+"\n", args...)
}

func (r registryPackagesProxyLogger) Warnf(format string, args ...interface{}) {
	log.WarnF(format+"\n", args...)
}

func (r registryPackagesProxyLogger) Debugf(format string, args ...interface{}) {
	log.DebugF(format+"\n", args...)
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

func RunBashiblePipeline(nodeInterface node.Interface, cfg *config.MetaConfig, nodeIP, devicePath string) error {
	var clusterDomain string
	err := json.Unmarshal(cfg.ClusterConfig["clusterDomain"], &clusterDomain)
	if err != nil {
		return err
	}

	log.DebugF("Got cluster domain: %s", clusterDomain)
	log.DebugLn("Starting registry packages proxy")

	// we need clusterDomain to generate proper certificate for packages proxy
	err = StartRegistryPackagesProxy(cfg.Registry, clusterDomain)
	if err != nil {
		return fmt.Errorf("failed to start registry packages proxy: %v", err)
	}

	if err := CheckDHCTLDependencies(nodeInterface); err != nil {
		return err
	}

	bundleName, err := DetermineBundleName(nodeInterface)
	if err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	log.DebugF("Rendered templates directory %s\n", templateController.TmpDir)

	err = log.Process("bootstrap", "Preparing bootstrap", func() error {
		if err := template.PrepareBootstrap(templateController, nodeIP, bundleName, cfg); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		err := retry.NewLoop(fmt.Sprintf("Prepare %s", app.NodeDeckhouseDirectoryPath), 30, 10*time.Second).Run(func() error {
			cmd := nodeInterface.Command("mkdir", "-p", "-m", "0755", app.DeckhouseNodeBinPath)
			cmd.Sudo()
			if err = cmd.Run(); err != nil {
				return fmt.Errorf("ssh: mkdir -p %s -m 0755: %w", app.DeckhouseNodeBinPath, err)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("cannot create %s directories: %w", app.NodeDeckhouseDirectoryPath, err)
		}

		err = retry.NewLoop(fmt.Sprintf("Prepare %s", app.DeckhouseNodeTmpPath), 30, 10*time.Second).Run(func() error {
			cmd := nodeInterface.Command("mkdir", "-p", "-m", "1777", app.DeckhouseNodeTmpPath)
			cmd.Sudo()
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("ssh: mkdir -p -m 1777 %s: %w", app.DeckhouseNodeTmpPath, err)
			}

			return nil
		})

		// in end of pipeline steps bashible write "OK" to this file
		// we need creating it before because we do not want handle errors from cat
		return retry.NewLoop(fmt.Sprintf("Prepare %s", DHCTLEndBootstrapBashiblePipeline), 30, 10*time.Second).Run(func() error {
			cmd := nodeInterface.Command("touch", DHCTLEndBootstrapBashiblePipeline)
			cmd.Sudo()
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("touch error %s: %w", DHCTLEndBootstrapBashiblePipeline, err)
			}

			return nil
		})
	})

	if wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper); ok {
		cleanUpTunnel, err := setupRPPTunnel(wrapper.Client())
		if err != nil {
			return err
		}

		defer cleanUpTunnel()
	}

	if err = PrepareBashibleBundle(bundleName, nodeIP, devicePath, cfg, templateController); err != nil {
		return err
	}
	tomb.RegisterOnShutdown("Delete templates temporary directory", func() {
		if !app.IsDebug {
			_ = os.RemoveAll(templateController.TmpDir)
		}
	})

	if err := BootstrapMaster(nodeInterface, templateController); err != nil {
		return err
	}

	return retry.NewLoop("Execute bundle", 30, 10*time.Second).
		BreakIf(func(err error) bool { return errors.Is(err, frontend.ErrBashibleTimeout) }).
		Run(func() error {
			// we do not need to restart tunnel because we have HealthMonitor

			log.DebugLn("Check bundle routine start")
			ready, err := checkBashibleAlreadyRun(nodeInterface)
			if err != nil {
				return err
			}

			if ready {
				log.Success("Bashible already run!\n")
				return nil
			}

			if err := cleanupPreviousBashibleRunIfNeed(nodeInterface); err != nil {
				return err
			}

			log.DebugLn("Start execute bashible bundle routine")

			return ExecuteBashibleBundle(nodeInterface, templateController.TmpDir)
		})
}

func setupRPPTunnel(sshClient *ssh.Client) (func(), error) {
	var tun *frontend.ReverseTunnel
	log.DebugLn("Starting reverse tunnel routine")
	tun, err := SetupSSHTunnelToRegistryPackagesProxy(sshClient)
	if err != nil {
		return nil, fmt.Errorf("failed to setup SSH tunnel to registry packages proxy: %v", err)
	}

	cleanUpTunnel := func() {
		if tun == nil {
			log.DebugLn("tun == nil. Skip cleanup tunnel")
			return
		}

		tun.Stop()
		tun = nil
	}
	return cleanUpTunnel, nil
}

func CheckDHCTLDependencies(nodeInteface node.Interface) error {

	type checkResult struct {
		name string
		err  error
	}

	checkDependency := func(dep string, resultsChan chan checkResult) error {
		breakPredicate := func(err error) bool {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				if ee.ExitCode() == 255 {
					return false
				}
			}
			return true
		}

		return retry.NewSilentLoop(fmt.Sprintf("Check dependency %s", dep), 30, 5*time.Second).BreakIf(breakPredicate).Run(func() error {
			output, err := nodeInteface.Command("command", "-v", dep).CombinedOutput()

			if err != nil {
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					log.DebugF("exit code: %v", ee)
				}
				e := fmt.Errorf("bashible dependency %s error: %v - %s",
					dep,
					err,
					string(output),
				)
				resultsChan <- checkResult{
					name: dep,
					err:  e,
				}
				log.DebugF("Dependency check error: %v\n", e)
				return e
			}
			return nil
		})
	}

	return log.Process("bootstrap", "Check DHCTL Dependencies", func() error {
		dependencyCommands := [][]string{
			{"sudo", "rm", "tar", "mount", "awk"},
			{"grep", "cut", "sed", "shopt", "mkdir"},
			{"cp", "join", "cat", "ps", "kill"},
		}

		resultsChan := make(chan checkResult)

		exceedDependency := errors.New("All dependency checks was exceed")

		go func() {
			wg := sync.WaitGroup{}
			for _, deps := range dependencyCommands {
				for _, dep := range deps {
					wg.Add(1)
					dep := dep
					log.InfoF("Check '%s' dependency\n", dep)
					go func() {
						defer wg.Done()
						err := checkDependency(dep, resultsChan)

						if err != nil {
							err = errors.Join(exceedDependency, err)
						}

						resultsChan <- checkResult{
							name: dep,
							err:  err,
						}
					}()
				}
				time.Sleep(1 * time.Second)
			}
			log.DebugLn("Wait all dependency checks successful")
			wg.Wait()
			log.DebugLn("Close result chan")
			close(resultsChan)
		}()

		for res := range resultsChan {
			if res.err != nil {
				if errors.Is(res.err, exceedDependency) {
					return res.err
				}
				log.WarnLn(res.err)
				continue
			}
			log.Success(fmt.Sprintf("Dependency '%s' check success\n", res.name))
		}

		log.InfoLn("OK!")
		return nil

	})
}

func DetermineBundleName(nodeInterface node.Interface) (string, error) {
	var bundleName string
	err := log.Process("bootstrap", "Detect Bashible Bundle", func() error {
		file, err := template.RenderAndSaveDetectBundle(make(map[string]interface{}))
		if err != nil {
			return err
		}

		return retry.NewSilentLoop("Get bundle", 30, 10*time.Second).Run(func() error {
			// run detect bundle type
			detectCmd := nodeInterface.UploadScript(file)
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

func GetMasterHostsIPs() ([]session.Host, error) {
	var hosts map[string]string
	err := cache.Global().LoadStruct(MasterHostsCacheKey, &hosts)
	if err != nil {
		return nil, err
	}
	mastersIPs := make([]session.Host, 0, len(hosts))
	for name, ip := range hosts {
		mastersIPs = append(mastersIPs, session.Host{Host: ip, Name: name})
	}

	sort.Sort(session.SortByName(mastersIPs))

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
