// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registrydata

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

// GetRegistryDataPreferUpstream resolves the registry to pull images from. Out
// of the cluster (inCluster=false: manual dhctl over SSH, commander) it prefers
// the upstream registry from registry-config, which is reachable from anywhere,
// because the deckhouse-registry mirror (registry.d8-system.svc) only resolves
// inside the cluster. In the cluster (auto-converger, exporter) the mirror is
// the fast local path, so it is used directly. Falls back to the mirror when no
// upstream is configured (older clusters that expose the reachable registry in
// deckhouse-registry directly).
func GetRegistryDataPreferUpstream(ctx context.Context, kubeCl *client.KubernetesClient, inCluster bool) (*image.RegistryConfig, string, error) {
	if inCluster {
		return GetRegistryData(ctx, kubeCl)
	}

	conf, dockerCfg, found, err := getUpstreamRegistryData(ctx, kubeCl)
	if err != nil {
		return nil, "", err
	}
	if found {
		return conf, dockerCfg, nil
	}

	return GetRegistryData(ctx, kubeCl)
}

var (
	d8RppSecretName      = "deckhouse-registry"
	d8RppSecretNamespace = "d8-system"
	registryConfigSecret = "registry-config"
)

func GetRegistryData(ctx context.Context, kubeCl *client.KubernetesClient) (*image.RegistryConfig, string, error) {
	conf := &image.RegistryConfig{}
	var b64dc string

	err := retry.NewLoop("Get registry data from cluster", 225, 1*time.Second).RunContext(ctx, func() error {
		secret, err := kubeCl.CoreV1().
			Secrets(d8RppSecretNamespace).
			Get(ctx, d8RppSecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if secret.Data[".dockerconfigjson"] != nil {
			b64dc = base64.StdEncoding.EncodeToString(secret.Data[".dockerconfigjson"])
			dc, err := image.ParseDockerConfig(secret.Data[".dockerconfigjson"])
			if err != nil {
				return err
			}
			registry := string(secret.Data["imagesRegistry"])
			scheme := strings.ToUpper(string(secret.Data["scheme"]))

			conf, err = image.RegistryConfigFromDockerConfig(dc, scheme, registry)
			if err != nil {
				return err
			}
		}
		if secret.Data["ca"] != nil {
			conf.SetCA(string(secret.Data["ca"]))
		}

		return nil
	})

	return conf, b64dc, err
}

// GetUpstreamRegistryData reads the upstream (externally reachable) registry from
// the d8-system/registry-config secret. On clusters running an in-cluster
// registry (Direct/Proxy modes) the deckhouse-registry secret points at the
// in-cluster mirror (registry.d8-system.svc), which an out-of-cluster caller
// (the commander dhctl-server) cannot resolve; the upstream imagesRepo is the
// registry it must pull from. found is false when the secret is absent (older
// clusters without the registry module) or carries no imagesRepo (Local mode),
// so the caller can fall back to GetRegistryData.
func GetUpstreamRegistryData(ctx context.Context, kubeCl *client.KubernetesClient) (*image.RegistryConfig, bool, error) {
	conf, _, found, err := getUpstreamRegistryData(ctx, kubeCl)
	return conf, found, err
}

// getUpstreamRegistryData additionally builds the registryDockerCfg (base64
// docker config json) from the upstream credentials, so a caller that also
// needs the dockercfg for lazy image pulls does not fall back to the
// in-cluster mirror's credentials.
func getUpstreamRegistryData(ctx context.Context, kubeCl *client.KubernetesClient) (*image.RegistryConfig, string, bool, error) {
	var secret *corev1.Secret
	err := retry.NewLoop("Get upstream registry data from cluster", 225, 1*time.Second).
		BreakIf(apierrors.IsNotFound).
		RunContext(ctx, func() error {
			got, err := kubeCl.CoreV1().
				Secrets(d8RppSecretNamespace).
				Get(ctx, registryConfigSecret, metav1.GetOptions{})
			if err != nil {
				return err
			}
			secret = got
			return nil
		})
	if apierrors.IsNotFound(err) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}

	imagesRepo := string(secret.Data["imagesRepo"])
	if imagesRepo == "" {
		return nil, "", false, nil
	}

	scheme := strings.ToUpper(string(secret.Data["scheme"]))
	if scheme == "" {
		scheme = "HTTPS"
	}
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	conf, err := image.NewRegistryConfig(scheme, imagesRepo, username, password, string(secret.Data["ca"]))
	if err != nil {
		return nil, "", false, fmt.Errorf("build upstream registry config: %w", err)
	}

	address, _ := helpers.SplitAddressAndPath(imagesRepo)
	dockerCfg, err := helpers.DockerCfgFromCreds(username, password, address)
	if err != nil {
		return nil, "", false, fmt.Errorf("build upstream registry dockercfg: %w", err)
	}

	return conf, base64.StdEncoding.EncodeToString(dockerCfg), true, nil
}
