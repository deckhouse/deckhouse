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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"basic-auth-proxy/pkg/proxy"
)

func main() {
	handler := proxy.New()

	rootCmd := &cobra.Command{
		Use:           "basic-auth-proxy",
		Short:         "Basic auth proxy for Kubernetes API Server",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "--------------------------------")
			fmt.Fprintln(out, "[ Starting Basic auth proxy ]")
			fmt.Fprintln(out, "--------------------------------")

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return handler.Run(ctx)
		},
	}

	rootCmd.PersistentFlags().StringVar(&handler.ListenAddress, "listen", ":7332", "listen address and port")
	rootCmd.PersistentFlags().StringVar(&handler.CertPath, "cert-path", "/some/cert/path", "directory with client.crt and client.key files")
	rootCmd.PersistentFlags().StringVar(&handler.KubernetesAPIServerURL, "api-server-url", "https://kubernetes.default", "Kubernetes api server URL")
	rootCmd.PersistentFlags().DurationVar(&handler.AuthCacheTTL, "auth-cache-ttl", 10*time.Second, "Authentication cache TTL (applies to negative results)")
	rootCmd.PersistentFlags().DurationVar(&handler.GroupsCacheTTL, "groups-cache-ttl", 2*time.Minute, "Groups cache TTL (applies to successful authentication results)")

	rootCmd.PersistentFlags().StringVar(&handler.CrowdBaseURL, "crowd-base-url", "", "URL of Atlassian Crowd")
	rootCmd.PersistentFlags().StringVar(&handler.CrowdApplicationLogin, "crowd-application-login", "", "login of Atlassian Crowd application")
	rootCmd.PersistentFlags().StringVar(&handler.CrowdApplicationPassword, "crowd-application-password", "", "password of Atlassian Crowd application")
	rootCmd.PersistentFlags().StringArrayVar(&handler.CrowdGroups, "crowd-allowed-group", nil, "Allowed Crowd groups")

	rootCmd.PersistentFlags().BoolVar(&handler.OIDCBasicAuthUnsupported, "oidc-basic-auth-unsupported", false, "basicAuthUnsupported option")
	rootCmd.PersistentFlags().BoolVar(&handler.OIDCGetUserInfo, "oidc-get-user-info", false, "getUserInfo option")
	rootCmd.PersistentFlags().StringVar(&handler.OIDCBaseURL, "oidc-base-url", "", "URL of OIDC provider")
	rootCmd.PersistentFlags().StringVar(&handler.OIDCClientID, "oidc-client-id", "", "clientID of OIDC application")
	rootCmd.PersistentFlags().StringVar(&handler.OIDCClientSecret, "oidc-client-secret", "", "clientSecret of OIDC application")
	rootCmd.PersistentFlags().StringArrayVar(&handler.OIDCScopes, "oidc-scope", nil, "Scopes passed from OIDC provider settings")

	rootCmd.PersistentFlags().BoolVar(&handler.LDAPBasicAuthUnsupported, "ldap-basic-auth-unsupported", false, "basicAuthUnsupported option for LDAP OIDC provider")
	rootCmd.PersistentFlags().BoolVar(&handler.LDAPGetUserInfo, "ldap-include-user-info-claims", false, "include user information claims from LDAP OIDC provider")
	rootCmd.PersistentFlags().StringVar(&handler.LDAPBaseURL, "ldap-base-url", "", "URL of LDAP OIDC provider (Dex)")
	rootCmd.PersistentFlags().StringVar(&handler.LDAPClientID, "ldap-client-id", "", "clientID of LDAP OIDC application")
	rootCmd.PersistentFlags().StringVar(&handler.LDAPClientSecret, "ldap-client-secret", "", "clientSecret of LDAP OIDC application")
	rootCmd.PersistentFlags().StringArrayVar(&handler.LDAPScopes, "ldap-scope", nil, "Scopes passed from LDAP OIDC provider settings")

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintf(rootCmd.ErrOrStderr(), "basic-auth-proxy: %v\n", err)
		os.Exit(1)
	}
}
