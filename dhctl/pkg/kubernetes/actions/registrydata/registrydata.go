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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
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

	// The cluster's own configuration object, asked first, because on a cluster running the
	// controller-based implementation it is the only source that is kept current.
	//
	// `registry-config` below is rendered from the PREVIOUS implementation's settings in
	// `mc/deckhouse`, and on a migrated cluster nobody writes those any more — so it keeps describing
	// whatever registry the cluster was migrated from. Measured on a cluster whose upstream had been
	// moved to `dev-registry.deckhouse.io/sys/deckhouse-oss`: the secret still said
	// `111.88.253.76.sslip.io/dh-dev-registry/sys/deckhouse-oss` with that mirror's robot credentials,
	// and this function preferred it. Stale registry data, preferred by the tooling, is worse than no
	// data at all: absence falls back and works, staleness dials the wrong registry with the wrong
	// account.
	if conf, dockerCfg, found, err := registryDataFromConfigResource(ctx, kubeCl); err != nil {
		return nil, "", err
	} else if found {
		return conf, dockerCfg, nil
	}

	conf, dockerCfg, found, err := getUpstreamRegistryData(ctx, kubeCl)
	if err != nil {
		return nil, "", err
	}
	if found {
		return conf, dockerCfg, nil
	}

	// No upstream, which for an air-gapped cluster is not a gap in its configuration but the point of
	// it. What is left is the cluster's own store, reachable over the SSH connection that is already
	// open to the master — see mirrorThroughNode, which returns its own docker config as well as its own
	// address, because both the host a docker config is keyed by and the account it names are different
	// there.
	conf, dockerCfg, err = GetRegistryData(ctx, kubeCl)
	if err != nil {
		return nil, "", err
	}

	if throughNode, throughNodeCfg, ok, err := mirrorThroughNode(ctx, kubeCl, conf); err != nil {
		return nil, "", err
	} else if ok {
		return throughNode, throughNodeCfg, nil
	}

	return conf, dockerCfg, nil
}

var (
	d8RppSecretName      = "deckhouse-registry"
	d8RppSecretNamespace = "d8-system"
	registryConfigSecret = "registry-config"

	// registryConfigResourceName is the singleton the controller-based implementation resolves its
	// configuration into.
	registryConfigResourceName = "registry"
)

// ErrRegistryDataTransient marks a transport/API-level failure while reading the registry
// secret, as opposed to a permanent parse failure of the secret's own content (malformed
// docker config, bad scheme/credentials) that will fail identically on every attempt.
var ErrRegistryDataTransient = fmt.Errorf("registry data: transient error, may succeed on retry")

// registryDataFromConfigResource reads the upstream from the cluster's `RegistryConfig`, which is what
// the controller-based implementation resolves its configuration into and keeps current.
//
// `found` is false, without an error, in every case where this object cannot answer the question: no
// such resource (a cluster on the previous implementation, or one too old to have it), or a resource
// with no upstream at all — which is not a gap but the definition of an air-gapped cluster, and the
// caller then falls back to the store reached through the SSH connection.
func registryDataFromConfigResource(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) (*image.RegistryConfig, string, bool, error) {
	gvr := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "registryconfigs",
	}

	var object *unstructured.Unstructured
	// Five attempts and not the forty-five the secret gets, because this is a PREFERENCE and not a
	// requirement: what cannot be read quickly is not worth waiting for when two working fallbacks sit
	// below it. Measured while writing this: with a long loop the absent-resource path added
	// forty-five seconds to every call on a cluster that simply has no such kind.
	err := retry.NewLoop("Get registry configuration from cluster", 5, 1*time.Second).
		BreakIf(apierrors.IsNotFound).
		BreakIf(meta.IsNoMatchError).
		RunContext(ctx, func() error {
			got, err := kubeCl.Dynamic().Resource(gvr).Get(ctx, registryConfigResourceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			object = got
			return nil
		})
	switch {
	case apierrors.IsNotFound(err), meta.IsNoMatchError(err):
		// No resource, or no such kind in this cluster at all: both mean "ask something else".
		return nil, "", false, nil
	case err != nil:
		return nil, "", false, err
	}

	upstream, ok, err := unstructured.NestedMap(object.Object, "spec", "primary", "upstream")
	if err != nil {
		return nil, "", false, fmt.Errorf("read the upstream from RegistryConfig: %w", err)
	}
	if !ok {
		return nil, "", false, nil
	}

	host, _ := upstream["host"].(string)
	if host == "" {
		return nil, "", false, nil
	}
	path, _ := upstream["path"].(string)
	ca, _ := upstream["ca"].(string)

	scheme := "HTTPS"
	if declared, _ := upstream["scheme"].(string); declared != "" {
		scheme = strings.ToUpper(declared)
	}

	// The credentials as the resource carries them: a username and password pair, or the pre-encoded
	// form a user typed directly. Split rather than passed through, because what is built below encodes
	// the pair itself.
	var username, password string
	if auth, found := upstream["auth"].(map[string]interface{}); found {
		username, _ = auth["username"].(string)
		password, _ = auth["password"].(string)

		if username == "" {
			if encoded, _ := auth["auth"].(string); encoded != "" {
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					return nil, "", false, fmt.Errorf("decode the upstream credentials: %w", err)
				}
				if user, pass, cut := strings.Cut(string(decoded), ":"); cut {
					username, password = user, pass
				}
			}
		}
	}

	conf, err := image.NewRegistryConfig(scheme, host+path, username, password, ca)
	if err != nil {
		return nil, "", false, fmt.Errorf("build registry config from RegistryConfig: %w", err)
	}

	dockerCfg, err := helpers.DockerCfgFromCreds(username, password, host)
	if err != nil {
		return nil, "", false, fmt.Errorf("build registry dockercfg from RegistryConfig: %w", err)
	}

	return conf, base64.StdEncoding.EncodeToString(dockerCfg), true, nil
}

func GetRegistryData(ctx context.Context, kubeCl *client.KubernetesClient) (*image.RegistryConfig, string, error) {
	conf := &image.RegistryConfig{}
	var b64dc string

	loopParams := retry.NewEmptyParams(
		retry.WithName("Get registry data from cluster"),
		retry.WithAttempts(225),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(ErrRegistryDataTransient),
	)

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		secret, err := kubeCl.CoreV1().
			Secrets(d8RppSecretNamespace).
			Get(ctx, d8RppSecretName, metav1.GetOptions{})
		if err != nil {
			if kubeerrors.IsPermanentAuthError(ctx, err) {
				return err
			}
			return fmt.Errorf("%w: %w", ErrRegistryDataTransient, err)
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
