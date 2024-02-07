/*
Copyright 2021 Flant JSC

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
	rootCmd.PersistentFlags().DurationVar(&handler.AuthCacheTTL, "auth-cache-ttl", 10*time.Second, "Crowd auth cache TTL")
	rootCmd.PersistentFlags().DurationVar(&handler.GroupsCacheTTL, "groups-cache-ttl", 2*time.Minute, "Crowd groups cache TTL")

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("starting crowd proxy error: %s", err)
		os.Exit(1)
	}
}
