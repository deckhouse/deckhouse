package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"crowd-auth-proxy/pkg/proxy"
)

var (
	listenAddress            string
	kubernetesApiServerURL   string
	certPath                 string
	crowdBaseUrl             string
	crowdApplicationLogin    string
	crowdApplicationPassword string
	cacheTTL                 int
)

var rootCmd = &cobra.Command{
	Use:   "crowd-auth-proxy",
	Short: "Basic auth proxy for Kubernetes Api Server with Atlassian Crowd",
	Long:  `Basic auth proxy for Kubernetes Api Server with Atlassian Crowd`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("-------------------------------------")
		fmt.Println("[😈 Starting Crowd Basic auth proxy ]")
		fmt.Println("-------------------------------------")
		proxy.RunProxy(listenAddress, kubernetesApiServerURL, crowdApplicationLogin, crowdApplicationPassword, certPath, crowdBaseUrl, cacheTTL)
	},
}

func Execute() {
	rootCmd.PersistentFlags().StringVar(&listenAddress, "listen", ":7332", "listen address and port")
	rootCmd.PersistentFlags().StringVar(&certPath, "cert-path", "/some/cert/path", "directory with client.crt and client.key files")
	rootCmd.PersistentFlags().StringVar(&crowdBaseUrl, "crowd-base-url", "https://crowd.example.com", "URL of Atlassian Crowd")
	rootCmd.PersistentFlags().StringVar(&crowdApplicationLogin, "crowd-application-login", "crowd", "login of Atlassian Crowd application")
	rootCmd.PersistentFlags().StringVar(&crowdApplicationPassword, "crowd-application-password", "user123", "password of Atlassian Crowd application")
	rootCmd.PersistentFlags().StringVar(&kubernetesApiServerURL, "api-server-url", "https://api.example.com", "Kubernetes api server URL")
	rootCmd.PersistentFlags().IntVar(&cacheTTL, "cache-ttl", 10, "cache TTL in seconds")

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("starting crowd proxy error: %s", err)
		os.Exit(1)
	}
}
