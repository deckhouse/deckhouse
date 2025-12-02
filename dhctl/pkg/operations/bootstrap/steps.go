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
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	tplt "text/template"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	ManifestCreatedInClusterCacheKey  = "tf-state-and-manifests-in-cluster"
	BastionHostCacheKey               = "bastion-hosts"
	DHCTLEndBootstrapBashiblePipeline = app.NodeDeckhouseDirectoryPath + "/first-control-plane-bashible-ran"
)

func BootstrapMaster(ctx context.Context, nodeInterface node.Interface, controller *template.Controller) error {
	return log.Process("bootstrap", "Initial bootstrap", func() error {
		for _, bootstrapScript := range []string{"01-network-scripts.sh", "02-base-pkgs.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)

			err := retry.NewLoop(fmt.Sprintf("Execute %s", bootstrapScript), 30, 5*time.Second).
				RunContext(ctx, func() error {
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

					_, err := cmd.Execute(ctx)
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

func PrepareBashibleBundle(nodeIP, devicePath string, metaConfig *config.MetaConfig, controller *template.Controller) error {
	return log.Process("bootstrap", "Prepare Bashible", func() error {
		return template.PrepareBundle(controller, nodeIP, devicePath, metaConfig)
	})
}

func ExecuteBashibleBundle(ctx context.Context, nodeInterface node.Interface, tmpDir string, commanderMode bool) error {
	bundleCmd := nodeInterface.UploadScript("bashible.sh", "--local")
	bundleCmd.WithCommanderMode(commanderMode)
	bundleCmd.WithCleanupAfterExec(false)
	bundleCmd.Sudo()
	parentDir := tmpDir + "/var/lib"
	bundleDir := "bashible"

	_, err := bundleCmd.ExecuteBundle(ctx, parentDir, bundleDir)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("bundle '%s' error: %v\nstderr: %s", bundleDir, err, string(ee.Stderr))
		}

		if errors.Is(err, frontend.ErrBashibleTimeout) {
			return frontend.ErrBashibleTimeout
		}

		if errors.Is(err, gossh.ErrBashibleTimeout) {
			return gossh.ErrBashibleTimeout
		}

		return fmt.Errorf("bundle '%s' error: %v", bundleDir, err)
	}
	return nil
}

func checkBashibleAlreadyRun(ctx context.Context, nodeInterface node.Interface) (bool, error) {
	isReady := false
	err := log.Process("bootstrap", "Checking bashible is ready", func() error {
		cmd := nodeInterface.Command("cat", DHCTLEndBootstrapBashiblePipeline)
		cmd.Sudo(ctx)
		cmd.WithTimeout(10 * time.Second)
		stdout, stderr, err := cmd.Output(ctx)
		if err != nil {
			isReady = false
			return err
		}

		log.DebugF("cat %s stdout: '%s'; stderr: '%s'\n", DHCTLEndBootstrapBashiblePipeline, stdout, stderr)

		isReady = strings.TrimSpace(string(stdout)) == "OK"

		return nil
	})

	return isReady, err
}

func getBashiblePIDs(ctx context.Context, nodeInterface node.Interface) ([]string, error) {
	var psStrings []string
	h := func(l string) {
		psStrings = append(psStrings, l)
	}
	cmd := nodeInterface.Command("bash", "-c", `ps a --no-headers -o args:64 -o "|%p"`)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	cmd.WithStdoutHandler(h)
	if err := cmd.Run(ctx); err != nil {
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

func killBashible(ctx context.Context, nodeInterface node.Interface, pids []string) error {
	cmd := nodeInterface.Command("kill", pids...)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(ctx); err != nil {
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

func unlockBashible(ctx context.Context, NodeInterface node.Interface) error {
	cmd := NodeInterface.Command("rm", "-f", "/var/lock/bashible")
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(ctx); err != nil {
		return err
	}

	return nil
}

func cleanupPreviousBashibleRunIfNeed(ctx context.Context, nodeInterface node.Interface) error {
	return log.Process("bootstrap", "Cleanup previous bashible run if need", func() error {
		log.DebugF("Gettting bashible pids")
		pids, err := getBashiblePIDs(ctx, nodeInterface)
		if err != nil {
			return err
		}

		log.DebugLn("Got bashible pids: %v", pids)
		if len(pids) == 0 {
			log.InfoLn("Bashible instance not found. Start it!")
			return nil
		}

		if err := killBashible(ctx, nodeInterface, pids); err != nil {
			return err
		}

		return unlockBashible(ctx, nodeInterface)
	})
}

func SetupSSHTunnelToRegistryPackagesProxy(ctx context.Context, sshCl node.SSHClient) (node.ReverseTunnel, error) {
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

	tun.StartHealthMonitor(ctx, checker, killer)

	return tun, nil
}

type registryClientConfigGetter struct {
	registry.ClientConfig
}

func newRegistryClientConfigGetter(config registry_config.Data) (*registryClientConfigGetter, error) {
	return &registryClientConfigGetter{
		ClientConfig: registry.ClientConfig{
			Repository: config.ImagesRepo,
			Scheme:     strings.ToLower(string(config.Scheme)),
			CA:         config.CA,
			Auth:       config.AuthBase64(),
		},
	}, nil
}

func (r *registryClientConfigGetter) Get(_ string) (*registry.ClientConfig, error) {
	return &r.ClientConfig, nil
}

func StartRegistryPackagesProxy(ctx context.Context, registryRemote registry_config.Data, rppSignCheck string, clusterDomain string) error {
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

	clientConfigGetter, err := newRegistryClientConfigGetter(registryRemote)
	if err != nil {
		return fmt.Errorf("Failed to create registry client for registry proxy: %v", err)
	}
	srv := &http.Server{}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	proxyConfig := &proxy.Config{SignCheck: (rppSignCheck == "true")}
	proxy := proxy.NewProxy(srv, listener, clientConfigGetter, registryPackagesProxyLogger{}, &registry.DefaultClient{})

	go proxy.Serve(proxyConfig)

	go func() {
		<-ctx.Done()
		proxy.StopProxy()
	}()

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

func (r registryPackagesProxyLogger) Error(msg string, args ...interface{}) {
	log.ErrorLn(msg, args)
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

func RunBashiblePipeline(ctx context.Context, nodeInterface node.Interface, cfg *config.MetaConfig, nodeIP, devicePath string, commanderMode bool) error {
	var clusterDomain string
	err := json.Unmarshal(cfg.ClusterConfig["clusterDomain"], &clusterDomain)
	if err != nil {
		return err
	}

	log.DebugF("Got cluster domain: %s", clusterDomain)

	if err := CheckDHCTLDependencies(ctx, nodeInterface); err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	log.DebugF("Rendered templates directory %s\n", templateController.TmpDir)

	err = log.Process("bootstrap", "Preparing bootstrap", func() error {
		if err := template.PrepareBootstrap(templateController, nodeIP, cfg); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		err := retry.NewLoop(fmt.Sprintf("Prepare %s", app.NodeDeckhouseDirectoryPath), 30, 10*time.Second).RunContext(ctx, func() error {
			cmd := nodeInterface.Command("sh", "-c", fmt.Sprintf("umask 0022 ; mkdir -p -m 0755 %s", app.DeckhouseNodeBinPath))
			cmd.Sudo(ctx)
			if err = cmd.Run(ctx); err != nil {
				return fmt.Errorf("ssh: mkdir -p %s -m 0755: %w", app.DeckhouseNodeBinPath, err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot create %s directories: %w", app.NodeDeckhouseDirectoryPath, err)
		}

		err = retry.NewLoop(fmt.Sprintf("Prepare %s", app.DeckhouseNodeTmpPath), 30, 10*time.Second).RunContext(ctx, func() error {
			cmd := nodeInterface.Command("sh", "-c", fmt.Sprintf("umask 0022 ; mkdir -p -m 1777 %s", app.DeckhouseNodeTmpPath))
			cmd.Sudo(ctx)
			if err := cmd.Run(ctx); err != nil {
				return fmt.Errorf("ssh: mkdir -p -m 1777 %s: %w", app.DeckhouseNodeTmpPath, err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot create %s directories: %w", app.DeckhouseNodeTmpPath, err)
		}

		// in end of pipeline steps bashible write "OK" to this file
		// we need creating it before because we do not want handle errors from cat
		return retry.NewLoop(fmt.Sprintf("Prepare %s", DHCTLEndBootstrapBashiblePipeline), 30, 10*time.Second).RunContext(ctx, func() error {
			cmd := nodeInterface.Command("sh", "-c", fmt.Sprintf("umask 0022 ; touch %s", DHCTLEndBootstrapBashiblePipeline))
			cmd.Sudo(ctx)
			if err := cmd.Run(ctx); err != nil {
				return fmt.Errorf("touch error %s: %w", DHCTLEndBootstrapBashiblePipeline, err)
			}
			return nil
		})
	})
	if err != nil {
		return err
	}

	ready := false

	err = retry.NewLoop("Checking bashible already ran", 30, 10*time.Second).RunContext(ctx, func() error {
		log.DebugLn("Check bundle routine start")
		var err error

		ready, err = checkBashibleAlreadyRun(ctx, nodeInterface)

		return err
	})
	if err != nil {
		return err
	}

	if ready {
		log.Success("Bashible already run! Skip bashible install\n\n")
		return nil
	}

	log.DebugLn("Starting registry packages proxy")
	// we need clusterDomain to generate proper certificate for packages proxy
	err = StartRegistryPackagesProxy(ctx, cfg.Registry.Settings.Remote, config.RppSignCheck, clusterDomain)
	if err != nil {
		return fmt.Errorf("failed to start registry packages proxy: %v", err)
	}

	if wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper); ok {
		cleanUpTunnel, err := setupRPPTunnel(ctx, wrapper.Client())
		if err != nil {
			return err
		}

		defer cleanUpTunnel()
	}

	if err = PrepareBashibleBundle(nodeIP, devicePath, cfg, templateController); err != nil {
		return err
	}
	tomb.RegisterOnShutdown("Delete templates temporary directory", func() {
		if !app.IsDebug {
			_ = os.RemoveAll(templateController.TmpDir)
		}
	})

	if err := BootstrapMaster(ctx, nodeInterface, templateController); err != nil {
		return err
	}

	return retry.NewLoop("Execute bundle", 10, 10*time.Second).
		BreakIf(func(err error) bool {
			return errors.Is(err, frontend.ErrBashibleTimeout) || errors.Is(err, gossh.ErrBashibleTimeout)
		}).
		RunContext(ctx, func() error {
			// we do not need to restart tunnel because we have HealthMonitor

			log.DebugLn("Stop bashible if need")

			if err := cleanupPreviousBashibleRunIfNeed(ctx, nodeInterface); err != nil {
				return err
			}

			log.DebugLn("Start execute bashible bundle routine")

			return ExecuteBashibleBundle(ctx, nodeInterface, templateController.TmpDir, commanderMode)
		})
}

func setupRPPTunnel(ctx context.Context, sshClient node.SSHClient) (func(), error) {
	var tun node.ReverseTunnel
	log.DebugLn("Starting reverse tunnel routine")
	tun, err := SetupSSHTunnelToRegistryPackagesProxy(ctx, sshClient)
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

const dependencyCheckTemplate = `
for dep in {{range $i, $d := .Deps}}{{if $i}} {{end}}{{$d}}{{end}}; do
  if command -v "$dep" >/dev/null 2>&1; then
    echo "1 $dep"
  else
    echo "0 $dep"
  fi
done
`

func buildDependencyCheckScript(deps []string) (string, error) {
	tmpl, err := tplt.New("dep-check").Parse(dependencyCheckTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse dependency template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct {
		Deps []string
	}{Deps: deps})
	if err != nil {
		return "", fmt.Errorf("failed to render dependency template: %w", err)
	}

	return buf.String(), nil
}

func CheckDHCTLDependencies(ctx context.Context, nodeInterface node.Interface) error {
	dependencies := []string{
		"sudo", "rm", "tar", "mount", "awk",
		"grep", "cut", "sed", "mkdir", "cp",
		"join", "cat", "ps", "kill",
	}

	return log.Process("bootstrap", "Check DHCTL Dependencies", func() error {
		breakPredicate := func(err error) bool {
			// Retry only for transient SSH connection issues
			if err == nil {
				return true
			}
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() == 255 {
				log.WarnLn("SSH connection failed (exit 255), retrying in 5 seconds...\n")
				return false
			}
			return true
		}

		var lastErr error

		runErr := retry.NewSilentLoop("Check all DHCTL dependencies", 30, 5*time.Second).
			BreakIf(breakPredicate).
			RunContext(ctx, func() error {
				bashScript, err := buildDependencyCheckScript(dependencies)
				if err != nil {
					return fmt.Errorf("failed to build dependency check script: %w", err)
				}

				log.DebugF("Generated dependency check bash script:\n%s\n", bashScript)
				//Encode the script to avoid "\n" characters and safely pass it via SSH
				encoded := base64.StdEncoding.EncodeToString([]byte(bashScript))
				remoteCmd := fmt.Sprintf("echo %q | base64 -d | bash", encoded)
				cmd := nodeInterface.Command("bash", "-c", remoteCmd)

				output, err := cmd.CombinedOutput(ctx)
				if err != nil {
					var ee *exec.ExitError
					if errors.As(err, &ee) {
						log.DebugF("SSH exit code: %v\n", ee.ExitCode())
					}
					e := fmt.Errorf("remote dependency check failed: %w - %s", err, string(output))
					log.DebugF("Dependency check error: %v\n", e)
					return e
				}

				var missing []string
				scanner := bufio.NewScanner(bytes.NewReader(output))
				for scanner.Scan() {
					fields := strings.Fields(scanner.Text())
					if len(fields) != 2 {
						continue
					}
					status, dep := fields[0], fields[1]

					log.InfoF("Checking '%s' dependency\n", dep)
					if status == "1" {
						log.Success(fmt.Sprintf("Dependency '%s' is available\n", dep))
					} else {
						log.WarnLn(fmt.Sprintf("Dependency '%s' is missing!\n", dep))
						missing = append(missing, dep)
					}
				}

				if err := scanner.Err(); err != nil {
					lastErr = fmt.Errorf("failed to read dependency output: %w", err)
					return lastErr
				}

				if len(missing) > 0 {
					lastErr = fmt.Errorf("missing dependencies: %v", missing)
					return lastErr
				}

				log.InfoLn("All dependencies are present.")
				return nil
			})

		if runErr != nil {
			if lastErr != nil {
				log.DebugF("Dependency checks exceeded maximum retries.\n")
				return fmt.Errorf("dependency check failed after retries: %w", lastErr)
			}
			return runErr
		}

		return nil
	})
}

func WaitForSSHConnectionOnMaster(ctx context.Context, sshClient node.SSHClient) error {
	return log.Process("bootstrap", "Wait for SSH on Master become Ready", func() error {
		availabilityCheck := sshClient.Check()
		_ = log.Process("default", "Connection string", func() error {
			log.InfoLn(availabilityCheck.String())
			return nil
		})

		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(ctx); err != nil {
			return fmt.Errorf("await master to become available: %v", err)
		}
		return nil
	})
}

type InstallDeckhouseResult struct {
	ManifestResult *deckhouse.ManifestsResult
}

func InstallDeckhouse(ctx context.Context, kubeCl *client.KubernetesClient, config *config.DeckhouseInstaller, beforeDeckhouseTask func() error) (*InstallDeckhouseResult, error) {
	res := &InstallDeckhouseResult{}
	err := log.Process("bootstrap", "Install Deckhouse", func() error {
		err := CheckPreventBreakAnotherBootstrappedCluster(ctx, kubeCl, config)
		if err != nil {
			return err
		}

		resManifests, err := deckhouse.CreateDeckhouseManifests(ctx, kubeCl, config, beforeDeckhouseTask)
		if err != nil {
			return fmt.Errorf("deckhouse create manifests: %v", err)
		}

		res.ManifestResult = resManifests

		err = cache.Global().Save(ManifestCreatedInClusterCacheKey, []byte("yes"))
		if err != nil {
			return fmt.Errorf("set manifests in cluster flag to cache: %v", err)
		}

		err = deckhouse.WaitForReadiness(ctx, kubeCl)
		if err != nil {
			return fmt.Errorf("deckhouse install: %v", err)
		}

		err = registry_config.WaitForRegistryInitialization(ctx, kubeCl, config.Registry)
		if err != nil {
			return fmt.Errorf("registry initialization: %v", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return res, nil
}

func BootstrapTerraNodes(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraNodeGroups []config.TerraNodeGroupSpec, infrastructureContext *infrastructure.Context) error {
	return log.Process("bootstrap", "Create CloudPermanent NG", func() error {
		return operations.ParallelCreateNodeGroup(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext)
	})
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

func BootstrapAdditionalMasterNodes(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, addressTracker map[string]string, infrastructureContext *infrastructure.Context, stateCache state.Cache) error {
	if metaConfig.MasterNodeGroupSpec.Replicas == 1 {
		log.DebugF("Skip bootstrap additional master nodes because replicas == 1")
		return nil
	}

	return log.Process("bootstrap", "Bootstrap additional master nodes", func() error {
		masterCloudConfig, err := entity.GetCloudConfig(ctx, kubeCl, global.MasterNodeGroupName, global.ShowDeckhouseLogs, log.GetDefaultLogger())
		if err != nil {
			return err
		}

		for i := 1; i < metaConfig.MasterNodeGroupSpec.Replicas; i++ {
			outputs, err := operations.BootstrapAdditionalMasterNode(ctx, kubeCl, metaConfig, i, masterCloudConfig, false, infrastructureContext)
			if err != nil {
				return err
			}
			addressTracker[fmt.Sprintf("%s-master-%d", metaConfig.ClusterPrefix, i)] = outputs.MasterIPForSSH

			state.SaveMasterHostsToCache(stateCache, addressTracker)
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
		case strings.HasPrefix(name, string(infrastructure.BaseInfraStep)):
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

func applyPostBootstrapModuleConfigs(kubeCl *client.KubernetesClient, tasks []actions.ModuleConfigTask) error {
	for _, task := range tasks {
		err := retry.NewLoop(task.Title, 15, 5*time.Second).
			Run(func() error {
				return task.Do(kubeCl)
			})
		if err != nil {
			return err
		}
	}

	return nil
}

func RunPostInstallTasks(ctx context.Context, kubeCl *client.KubernetesClient, result *InstallDeckhouseResult) error {
	if result == nil {
		log.DebugF("Skip post install tasks because result is nil\n")
		return nil
	}

	return log.Process("bootstrap", "Run post bootstrap actions", func() error {
		return applyPostBootstrapModuleConfigs(kubeCl, result.ManifestResult.PostBootstrapMCTasks)
	})
}
