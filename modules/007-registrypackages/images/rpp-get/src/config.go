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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
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

func loadConfig(args []string) (config, error) {
	cli, err := parseArgs(args)
	if err != nil {
		return config{}, err
	}

	if err := cli.resolve(); err != nil {
		return config{}, err
	}
	if err := os.MkdirAll(cli.tempDir, 0o755); err != nil {
		return config{}, fmt.Errorf("create temp dir: %w", err)
	}

	return cli.config, nil
}

func defaultConfig() config {
	return config{
		tempDir:        defaultTempDir,
		installedStore: defaultInstalledStore,
		retries:        defaultRetries,
		retryDelay:     defaultRetryDelay,
	}
}

func parseArgs(args []string) (cliConfig, error) {
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

func (c *cliConfig) resolve() error {
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

	return c.resolveFromKube(context.Background())
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

// resolveFromKube fetches RPP endpoints and token from the Kubernetes API with retries
func (c *cliConfig) resolveFromKube(ctx context.Context) error {
	apiServerEndpoints := parseEndpoints(c.kubeAPIServerEndpoints)
	var lastErr error

	for attempt := 1; attempt <= kubeRetries; attempt++ {
		if attempt > 1 {
			log.Printf("kube retry %d/%d (previous error: %v)", attempt, kubeRetries, lastErr)
			if err := waitRetry(ctx, kubeRetryDelay); err != nil {
				return err
			}
		}

		kube, err := newKubeClient(apiServerEndpoints, attempt)
		if err != nil {
			lastErr = fmt.Errorf("init kube client: %w", err)
			if !shouldRetryKube(lastErr) {
				return lastErr
			}
			continue
		}

		endpoints, err := kube.GetEndpoints(ctx)
		if err != nil {
			lastErr = fmt.Errorf("get endpoints from kube: %w", err)
			if !shouldRetryKube(lastErr) {
				return lastErr
			}
			continue
		}

		token, err := kube.GetToken(ctx)
		if err != nil {
			lastErr = fmt.Errorf("get token from kube: %w", err)
			if !shouldRetryKube(lastErr) {
				return lastErr
			}
			continue
		}

		c.endpoints = endpoints
		c.token = token
		log.Printf("rpp endpoints obtained from kube: %v", c.endpoints)
		return nil
	}

	return lastErr
}
