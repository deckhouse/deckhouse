/*
Copyright 2026 Flant JSC

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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"rpp-get/kube"
)

type config struct {
	mode           string
	tempDir        string
	installedStore string
	retries        int
	retryDelay     time.Duration
	resultPath     string
	rppRepository  string
	rppPath        string
	force          bool
	endpoints      []string
	token          string
	packages       []string
}

type cliConfig struct {
	config
	rppEndpoints           string
	rppToken               string
	kubeAPIServerEndpoints string
}

func defaultConfig() config {
	return config{
		tempDir:        defaultTempDir,
		installedStore: defaultInstalledStore,
		retries:        defaultRetries,
		retryDelay:     defaultRetryDelay,
	}
}

func parseConfig(args []string) (cliConfig, error) {
	if len(args) == 0 {
		return cliConfig{}, fmt.Errorf("mode is required, expected one of: %s, %s, %s", modeFetch, modeInstall, modeUninstall)
	}

	cli := cliConfig{config: defaultConfig()}
	cli.mode = args[0]

	fs := flag.NewFlagSet("rpp-get", flag.ContinueOnError)
	fs.StringVar(&cli.tempDir, "temp-dir", cli.tempDir, "Temporary directory")
	fs.StringVar(&cli.resultPath, "result", "", "Path to result file")
	fs.StringVar(&cli.rppEndpoints, "rpp-endpoints", "", "Comma-separated RPP endpoints")
	fs.StringVar(&cli.rppToken, "rpp-token", "", "RPP bearer token")
	fs.StringVar(&cli.rppRepository, "rpp-repository", cli.rppRepository, "RPP repository override")
	fs.StringVar(&cli.rppPath, "rpp-path", cli.rppPath, "RPP additional path")
	fs.StringVar(&cli.kubeAPIServerEndpoints, "kube-apiserver-endpoints", "", "Comma-separated kube-apiserver endpoints for bootstrap-token mode")
	fs.BoolVar(&cli.force, "force", cli.force, "Force download and install even if package is already present")

	if err := fs.Parse(args[1:]); err != nil {
		return cliConfig{}, err
	}

	switch cli.mode {
	case modeFetch, modeInstall, modeUninstall:
	default:
		return cliConfig{}, fmt.Errorf("unknown mode %q, expected one of: %s, %s, %s", cli.mode, modeFetch, modeInstall, modeUninstall)
	}

	cli.packages = fs.Args()

	return cli, nil
}

func (c *cliConfig) resolve(ctx context.Context) error {
	if c.mode == modeUninstall {
		return nil
	}

	endpoints, endpointsConfigured := resolveEndpoints(c.rppEndpoints)
	token, tokenConfigured := resolveToken(c.rppToken)
	kubeAPIServerEndpoints, _ := resolveKubeAPIServerEndpoints(c.kubeAPIServerEndpoints)

	c.endpoints = endpoints
	c.token = token
	c.kubeAPIServerEndpoints = kubeAPIServerEndpoints

	if strings.TrimSpace(c.rppEndpoints) != "" {
		log.Printf("rpp endpoints obtained from flag: %v", c.endpoints)
	} else if value, ok := os.LookupEnv("PACKAGES_PROXY_ADDRESSES"); ok && strings.TrimSpace(value) != "" {
		log.Printf("rpp endpoints obtained from env: %v", c.endpoints)
	}

	if endpointsConfigured && tokenConfigured {
		return nil
	}

	return c.resolveFromKube(ctx)
}

func resolveEndpoints(value string) ([]string, bool) {
	if value = strings.TrimSpace(value); value != "" {
		return parseEndpoints(value), true
	}

	if value, ok := os.LookupEnv("PACKAGES_PROXY_ADDRESSES"); ok {
		if value = strings.TrimSpace(value); value != "" {
			return parseEndpoints(value), true
		}
	}

	return nil, false
}

func resolveToken(value string) (string, bool) {
	if value = strings.TrimSpace(value); value != "" {
		return value, true
	}

	if value, ok := os.LookupEnv("PACKAGES_PROXY_TOKEN"); ok {
		if value = strings.TrimSpace(value); value != "" {
			return value, true
		}
	}

	return "", false
}

func resolveKubeAPIServerEndpoints(value string) (string, bool) {
	if value = strings.TrimSpace(value); value != "" {
		return value, true
	}

	if value, ok := os.LookupEnv("PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS"); ok {
		if value = strings.TrimSpace(value); value != "" {
			return value, true
		}
	}

	return "", false
}

func (c *cliConfig) resolveFromKube(ctx context.Context) error {
	kubeClient, err := kube.NewKubeletClient()
	if err != nil && !errors.Is(err, kube.ErrNoConfig) {
		return fmt.Errorf("init kube client from kubelet config: %w", err)
	}
	if err == nil {
		log.Printf("rpp endpoints not provided via flag or env, querying kube-apiserver via kubelet config for pods app=registry-packages-proxy in d8-cloud-instance-manager")
		return c.retryKubeFetch(ctx, func(_ int) (kube.Client, error) {
			return kubeClient, nil
		})
	}

	apiServerEndpoints := parseEndpoints(c.kubeAPIServerEndpoints)
	if len(apiServerEndpoints) == 0 {
		return errNoBootstrapAPIServerEndpoints
	}

	log.Printf("rpp endpoints not provided and no kubelet config found, querying kube-apiserver directly via bootstrap endpoints %v for pods app=registry-packages-proxy in d8-cloud-instance-manager", apiServerEndpoints)
	return c.retryKubeFetch(ctx, func(attempt int) (kube.Client, error) {
		endpoint := apiServerEndpoints[(attempt-1)%len(apiServerEndpoints)]
		return kube.NewBootstrapClient(endpoint)
	})
}

func (c *cliConfig) retryKubeFetch(ctx context.Context, clientFn func(attempt int) (kube.Client, error)) error {
	var lastErr error
	for attempt := 1; attempt <= kubeRetries; attempt++ {
		if attempt > 1 {
			log.Printf("kube-apiserver request failed (attempt %d/%d): %s; retrying in %s",
				attempt-1, kubeRetries, friendlyKubeError(lastErr), kubeRetryDelay)
			if err := waitRetry(ctx, kubeRetryDelay); err != nil {
				return err
			}
		}
		client, err := clientFn(attempt)
		if err != nil {
			return fmt.Errorf("init kube client: %w", err)
		}
		if err := c.fetchFromKube(ctx, client); err != nil {
			lastErr = err
			if !kube.ShouldRetry(lastErr) {
				return lastErr
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("kube-apiserver unreachable after %d attempts: %s (last error: %w)",
		kubeRetries, friendlyKubeError(lastErr), lastErr)
}

func (c *cliConfig) fetchFromKube(ctx context.Context, kubeClient kube.Client) error {
	endpoints, err := kubeClient.GetRPPEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("get endpoints from kube: %w", err)
	}

	token, err := kubeClient.GetRPPToken(ctx)
	if err != nil {
		return fmt.Errorf("get token from kube: %w", err)
	}

	c.endpoints = endpoints
	c.token = token
	log.Printf("rpp endpoints obtained from kube: %v", c.endpoints)
	return nil
}

func friendlyKubeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "EOF"):
		return "connection closed by kube-apiserver before response (EOF) — apiserver may be down, restarting, or rejecting the connection"
	case strings.Contains(msg, "connection refused"):
		return "kube-apiserver refused the connection — apiserver is not listening on this address"
	case strings.Contains(msg, "no such host"):
		return "kube-apiserver host could not be resolved — check DNS or endpoint configuration"
	case strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "context deadline exceeded"):
		return "kube-apiserver did not respond in time — apiserver overloaded or network blocked"
	case strings.Contains(msg, "x509"), strings.Contains(msg, "certificate"):
		return "TLS handshake with kube-apiserver failed — check CA bundle"
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"):
		return "kube-apiserver rejected credentials — token missing or insufficient RBAC for pods in d8-cloud-instance-manager"
	default:
		return msg
	}
}
