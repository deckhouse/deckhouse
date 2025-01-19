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

	"github.com/spf13/cobra"

	"node-proxy-sidecar/internal/config"
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
	rootCmd.Flags().StringVar(&authModeStr, "auth-mode", "dev", "Authentication mode: dev, cert")
	rootCmd.Flags().StringSliceVar(&cfg.APIHosts, "api-host", []string{"https://127.0.0.1:6443"}, "Kubernetes API server host(s)")
	rootCmd.Flags().StringVar(&cfg.SocketPath, "socket-path", "/var/run/haproxy.sock", "Path to HAProxy socket")
	rootCmd.Flags().StringVar(&cfg.CertPath, "cert-path", "/etc/kubernetes/node-proxy/haproxy.pem", "Path to client certificate")
	rootCmd.Flags().StringVar(&cfg.KeyPath, "key-path", "/etc/kubernetes/node-proxy/haproxy.key", "Path to client key")
	rootCmd.Flags().StringVar(&cfg.CACertPath, "ca-cert-path", "/etc/kubernetes/node-proxy/ca.crt", "Path to CA certificate")
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func test(updatedList []string) {
	fmt.Println("Updated Endpoints List:", updatedList)
}

func run(cmd *cobra.Command, args []string) {
	cfg.AuthMode = config.ParseAuthMode(authModeStr)
	if cfg.AuthMode == config.AuthCert {
		if cfg.CertPath == "" || cfg.KeyPath == "" || cfg.CACertPath == "" {
			fmt.Println("--cert-path, --key-path --ca-cert-path required")
			cmd.Usage()
			return
		}
	}

	k8s := &k8s.Client{}
	k8sClient := k8s.NewClient(cfg)

	err := k8sClient.WatchEndpoints("default", "kubernetes", []string{"https"}, test)
	if err != nil {
		fmt.Println("Error watching endpoints:", err)
		return
	}
	select {}
}
