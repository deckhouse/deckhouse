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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

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
	secret, err := kubeCl.CoreV1().
		Secrets(d8RppSecretNamespace).
		Get(ctx, registryConfigSecret, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	imagesRepo := string(secret.Data["imagesRepo"])
	if imagesRepo == "" {
		return nil, false, nil
	}

	scheme := strings.ToUpper(string(secret.Data["scheme"]))
	if scheme == "" {
		scheme = "HTTPS"
	}
	conf, err := image.NewRegistryConfig(
		scheme,
		imagesRepo,
		string(secret.Data["username"]),
		string(secret.Data["password"]),
		string(secret.Data["ca"]),
	)
	if err != nil {
		return nil, false, err
	}
	return conf, true, nil
}
