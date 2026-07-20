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

package runtimeconfig

import (
	"fmt"
	"os"
	"strconv"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubeconfigEnv             = "BASHIBLE_KUBECONFIG"
	deckhouseReadyzEnabledEnv = "BASHIBLE_DECKHOUSE_READYZ_ENABLED"
)

type RuntimeConfig struct {
	KubeconfigPath         string
	DeckhouseReadyzEnabled bool
}

func Load() RuntimeConfig {
	return RuntimeConfig{
		KubeconfigPath:         os.Getenv(kubeconfigEnv),
		DeckhouseReadyzEnabled: boolEnv(deckhouseReadyzEnabledEnv, true),
	}
}

func (c RuntimeConfig) RESTConfig() (*rest.Config, error) {
	if c.KubeconfigPath == "" {
		return rest.InClusterConfig()
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", c.KubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig %q: %w", c.KubeconfigPath, err)
	}

	return cfg, nil
}

func (c RuntimeConfig) ExportKubeconfigToEnv() {
	if c.KubeconfigPath != "" {
		_ = os.Setenv("KUBECONFIG", c.KubeconfigPath)
	}
}

func boolEnv(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
