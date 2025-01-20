/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"node-proxy-sidecar/internal/config"
	"node-proxy-sidecar/internal/haproxy"
	"node-proxy-sidecar/internal/k8s"
)

var (
	Version     string
	cfg         config.Config
	authModeStr string
)

var rootCmd = &cobra.Command{
	Use:   "node-proxy-sidecar",
	Short: "Node Proxy Sidecar",
	Run:   run,
}

func init() {
	rootCmd.Flags().StringVar(&authModeStr, "auth-mode", "cert", "Authentication mode: dev, cert")
	rootCmd.Flags().StringSliceVar(&cfg.APIHosts, "api-host", []string{"https://127.0.0.1:6443"}, "Kubernetes API server host(s)")
	rootCmd.Flags().StringVar(&cfg.SocketPath, "socket-path", "/socket/haproxy.socket", "Path to HAProxy socket")
	rootCmd.Flags().StringVar(&cfg.CertPath, "cert-path", "/etc/kubernetes/node-proxy/haproxy.pem", "Path to client certificate")
	rootCmd.Flags().StringVar(&cfg.KeyPath, "key-path", "/etc/kubernetes/node-proxy/haproxy.pem", "Path to client key")
	rootCmd.Flags().StringVar(&cfg.CACertPath, "ca-cert-path", "/etc/kubernetes/node-proxy/ca.crt", "Path to CA certificate")
	rootCmd.Flags().StringVar(&cfg.HAProxyConfigurationFile, "ha-config-path", "/config/config.cfg", "Path HAProxy configuration file")
	rootCmd.Flags().StringVar(&cfg.HAProxyHAProxyBin, "ha-bin", "/bin/haproxy", "Path to HAProxy bin file")
	rootCmd.Flags().StringVar(&cfg.HAProxyTransactionsDir, "ha-transactions-dir", "/tmp/transactions-dir/", "Path to HA transactions dir")
	rootCmd.Flags().StringVar(&cfg.ConfigPath, "config", "/config/discovery.yaml", "Path to backends config")
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func run(cmd *cobra.Command, args []string) {
	cfg.AuthMode = config.ParseAuthMode(authModeStr)
	// if cfg.AuthMode == config.AuthCert {
	// 	if cfg.CertPath == "" || cfg.KeyPath == "" || cfg.CACertPath == "" {
	// 		fmt.Println("--cert-path, --key-path --ca-cert-path required")
	// 		cmd.Usage()
	// 		return
	// 	}
	// }
	configData, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	c := config.BackendConfig{}
	err = yaml.Unmarshal(configData, &c)
	if err != nil {
		log.Fatalf("Error unmarshaling config file: %v", err)
	}

	ha := &haproxy.Client{}
	haproxyClient, err := ha.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating HAProxy client: %v", err)
	}

	k := &k8s.Client{}
	k8sClient, err := k.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating K8S client: %v", err)
	}

	for _, backend := range c.Backends {

		err = k8sClient.WatchEndpoints(backend, haproxyClient.BackendSync)
		if err != nil {
			fmt.Println("Error watching endpoints:", err)
			return
		}
	}

	select {}
}
