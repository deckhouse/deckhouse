package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"crowd-auth-proxy/pkg/proxy"
)

func Execute() {
	handler := proxy.NewHandler()

	rootCmd := &cobra.Command{
		Use:   "crowd-auth-proxy",
		Short: "Basic auth proxy for Kubernetes API Server with Atlassian Crowd",
		Long:  `Basic auth proxy for Kubernetes API Server with Atlassian Crowd`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("------------------------------------")
			fmt.Println("[ Starting Crowd Basic auth proxy ]")
			fmt.Println("------------------------------------")
			handler.Run()
		},
	}

	rootCmd.PersistentFlags().StringVar(&handler.ListenAddress, "listen", ":7332", "listen address and port")
	rootCmd.PersistentFlags().StringVar(&handler.CertPath, "cert-path", "/some/cert/path", "directory with client.crt and client.key files")
	rootCmd.PersistentFlags().StringVar(&handler.CrowdBaseURL, "crowd-base-url", "https://crowd.example.com", "URL of Atlassian Crowd")
	rootCmd.PersistentFlags().StringVar(&handler.CrowdApplicationLogin, "crowd-application-login", "crowd", "login of Atlassian Crowd application")
	rootCmd.PersistentFlags().StringVar(&handler.CrowdApplicationPassword, "crowd-application-password", "user123", "password of Atlassian Crowd application")
	rootCmd.PersistentFlags().StringArrayVar(&handler.CrowdGroups, "crowd-allowed-group", nil, "Allowed Crowd groups")
	rootCmd.PersistentFlags().StringVar(&handler.KubernetesAPIServerURL, "api-server-url", "https://api.example.com", "Kubernetes api server URL")
	rootCmd.PersistentFlags().DurationVar(&handler.CacheTTL, "cache-ttl", 10*time.Second, "Crowd cache TTL")

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("starting crowd proxy error: %s", err)
		os.Exit(1)
	}
}
