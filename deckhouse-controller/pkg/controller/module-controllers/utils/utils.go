// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"
	"errors"
	"strings"

	"github.com/gofrs/uuid/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	deckhouseNamespace = "d8-system"

	deckhouseDiscoverySecret = "deckhouse-discovery"
)

// GenerateRegistryOptionsFromModuleSource fetches settings from ModuleSource and generate registry options from them
func GenerateRegistryOptionsFromModuleSource(ms *v1alpha1.ModuleSource, clusterUUID string, logger *log.Logger) []cr.Option {
	rconf := &RegistryConfig{
		DockerConfig: ms.Spec.Registry.DockerCFG,
		Scheme:       ms.Spec.Registry.Scheme,
		CA:           ms.Spec.Registry.CA,
		UserAgent:    clusterUUID,
	}

	return GenerateRegistryOptions(rconf, logger)
}

type RegistryConfig struct {
	DockerConfig string
	CA           string
	Scheme       string
	UserAgent    string
}

func GenerateRegistryOptions(ri *RegistryConfig, logger *log.Logger) []cr.Option {
	if ri.UserAgent == "" {
		if logger.Enabled(context.Background(), log.LevelDebug.Level()) {
			logger.Debug("got empty user agent")
		}

		ri.UserAgent = "deckhouse-controller"
	}

	opts := []cr.Option{
		cr.WithAuth(ri.DockerConfig),
		cr.WithUserAgent(ri.UserAgent),
		cr.WithCA(ri.CA),
		cr.WithInsecureSchema(strings.ToLower(ri.Scheme) == "http"),
	}

	return opts
}

type DeckhouseRegistrySecret struct {
	DockerConfig          string
	Address               string
	ClusterIsBootstrapped string
	ImageRegistry         string
	Path                  string
	Scheme                string
	CA                    string
}

var ErrCAFieldIsNotFound = errors.New("secret has no ca field")

func ParseDeckhouseRegistrySecret(data map[string][]byte) (*DeckhouseRegistrySecret, error) {
	var err error

	dockerConfig, ok := data[".dockerconfigjson"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no .dockerconfigjson field"))
	}

	address, ok := data["address"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no address field"))
	}

	clusterIsBootstrapped, ok := data["clusterIsBootstrapped"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no clusterIsBootstrapped field"))
	}

	imagesRegistry, ok := data["imagesRegistry"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no imagesRegistry field"))
	}

	path, ok := data["path"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no path field"))
	}

	scheme, ok := data["scheme"]
	if !ok {
		err = errors.Join(err, errors.New("secret has no scheme field"))
	}

	ca, ok := data["ca"]
	if !ok {
		err = errors.Join(err, ErrCAFieldIsNotFound)
	}

	return &DeckhouseRegistrySecret{
		DockerConfig:          string(dockerConfig),
		Address:               string(address),
		ClusterIsBootstrapped: string(clusterIsBootstrapped),
		ImageRegistry:         string(imagesRegistry),
		Path:                  string(path),
		Scheme:                string(scheme),
		CA:                    string(ca),
	}, err
}

func Update[Object client.Object](ctx context.Context, cli client.Client, object Object, updater func(obj Object) bool) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := cli.Get(ctx, client.ObjectKey{Name: object.GetName()}, object); err != nil {
				return err
			}
			if updater(object) {
				return cli.Update(ctx, object)
			}
			return nil
		})
	})
}

func UpdateStatus[Object client.Object](ctx context.Context, cli client.Client, object Object, updater func(obj Object) bool) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := cli.Get(ctx, client.ObjectKey{Name: object.GetName()}, object); err != nil {
				return err
			}
			if updater(object) {
				return cli.Status().Update(ctx, object)
			}
			return nil
		})
	})
}

// UpdatePolicy return policy for the module
// if no policy for the module, embeddedPolicy is returned
func UpdatePolicy(ctx context.Context, cli client.Client, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, moduleName string) (*v1alpha2.ModuleUpdatePolicy, error) {
	module := new(v1alpha1.Module)
	if err := cli.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	} else {
		if module.Properties.UpdatePolicy != "" {
			policy := new(v1alpha2.ModuleUpdatePolicy)
			if err = cli.Get(ctx, client.ObjectKey{Name: module.Properties.UpdatePolicy}, policy); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, err
				}
			} else {
				return policy, nil
			}
		}
	}
	return &v1alpha2.ModuleUpdatePolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha2.ModuleUpdatePolicyGVK.Kind,
			APIVersion: v1alpha2.ModuleUpdatePolicyGVK.GroupVersion().String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "", // special empty default policy, inherits Deckhouse settings for update mode
		},
		Spec: *embeddedPolicy.Get(),
	}, nil
}

func ModulePullOverrideExists(ctx context.Context, cli client.Client, sourceName, moduleName string) (bool, error) {
	mpos := new(v1alpha1.ModulePullOverrideList)
	if err := cli.List(ctx, mpos, client.MatchingLabels{"source": sourceName, "module": moduleName}, client.Limit(1)); err != nil {
		return false, err
	}
	return len(mpos.Items) > 0, nil
}

func GetClusterUUID(ctx context.Context, cli client.Client) string {
	// attempt to read the cluster UUID from a secret
	secret := new(corev1.Secret)
	if err := cli.Get(ctx, client.ObjectKey{Namespace: deckhouseNamespace, Name: deckhouseDiscoverySecret}, secret); err != nil {
		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	// generate a random UUID if the key is missing
	return uuid.Must(uuid.NewV4()).String()
}
