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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gofrs/uuid/v5"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/reginjector"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/updater"
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

// Update updates object with retryOnConflict to avoid conflict
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

// UpdateStatus updates object status with retryOnConflict to avoid conflict
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

// UpdatePolicy returns policy for the module, if no policy, embeddedPolicy is returned
func UpdatePolicy(ctx context.Context, cli client.Client, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, moduleName string) (*v1alpha2.ModuleUpdatePolicy, error) {
	module := new(v1alpha1.Module)
	if err := cli.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}
	} else {
		if module.Properties.UpdatePolicy != "" {
			policy := new(v1alpha2.ModuleUpdatePolicy)
			if err = cli.Get(ctx, client.ObjectKey{Name: module.Properties.UpdatePolicy}, policy); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("get the '%s' update policy: %w", moduleName, err)
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

// ModulePullOverrideExists checks if module pull override for the module exists
func ModulePullOverrideExists(ctx context.Context, cli client.Client, moduleName string) (bool, error) {
	mpo := new(v1alpha2.ModulePullOverride)
	if err := cli.Get(ctx, client.ObjectKey{Name: moduleName}, mpo); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// GetClusterUUID gets uuid from the secret or generate a new one
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

// EnableModule deletes old symlinks and creates a new one
func EnableModule(downloadedModulesDir, oldSymlinkPath, newSymlinkPath, modulePath string) error {
	// delete the old module symlink with diff version if exists
	if oldSymlinkPath != "" {
		if _, err := os.Lstat(oldSymlinkPath); err == nil {
			if err = os.Remove(oldSymlinkPath); err != nil {
				return fmt.Errorf("delete the '%s' old symlink: %w", oldSymlinkPath, err)
			}
		}
	}

	// delete the new module symlink
	if _, err := os.Lstat(newSymlinkPath); err == nil {
		if err = os.Remove(newSymlinkPath); err != nil {
			return fmt.Errorf("delete the '%s' new symlink: %w", newSymlinkPath, err)
		}
	}

	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(downloadedModulesDir, strings.TrimPrefix(modulePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return fmt.Errorf("the '%s' module absolute path not found", moduleAbsPath)
	}

	return os.Symlink(modulePath, newSymlinkPath)
}

// GetModuleSymlink walks over the root dir to find a module symlink by regexp
func GetModuleSymlink(rootPath, moduleName string) (string, error) {
	var symlinkPath string

	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, _ error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}
		symlinkPath = path
		return filepath.SkipDir
	})

	return symlinkPath, err
}

// EnsureModuleDocumentation creates or updates module documentation
func EnsureModuleDocumentation(
	ctx context.Context,
	cli client.Client,
	moduleName, moduleSource, moduleChecksum, moduleVersion, modulePath string,
	ownerRef metav1.OwnerReference,
) error {
	md := new(v1alpha1.ModuleDocumentation)
	if err := cli.Get(ctx, client.ObjectKey{Name: moduleName}, md); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the '%s' module documentation: %w", moduleName, err)
		}
		md = &v1alpha1.ModuleDocumentation{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleDocumentationGVK.Kind,
				APIVersion: v1alpha1.ModuleDocumentationGVK.GroupVersion().String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: moduleName,
				Labels: map[string]string{
					v1alpha1.ModuleReleaseLabelModule: moduleName,
					v1alpha1.ModuleReleaseLabelSource: moduleSource,
				},
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Spec: v1alpha1.ModuleDocumentationSpec{
				Version:  moduleVersion,
				Path:     modulePath,
				Checksum: moduleChecksum,
			},
		}

		if err = cli.Create(ctx, md); err != nil {
			return fmt.Errorf("create the '%s' module documentation: %w", md.Name, err)
		}
	}

	if md.Spec.Version != moduleVersion || md.Spec.Checksum != moduleChecksum {
		// update module documentation
		md.Spec.Path = modulePath
		md.Spec.Version = moduleVersion
		md.Spec.Checksum = moduleChecksum
		md.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

		if err := cli.Update(ctx, md); err != nil {
			return fmt.Errorf("update the '%s' module documentation: %w", md.Name, err)
		}
	}

	return nil
}

// GetNotificationConfig gets config from discovery secret
func GetNotificationConfig(ctx context.Context, cli client.Client) (updater.NotificationConfig, error) {
	secret := new(corev1.Secret)
	if err := cli.Get(ctx, client.ObjectKey{Name: deckhouseDiscoverySecret, Namespace: deckhouseNamespace}, secret); err != nil {
		return updater.NotificationConfig{}, fmt.Errorf("get secret: %w", err)
	}

	// TODO: remove this dependency
	jsonSettings, ok := secret.Data["updateSettings.json"]
	if !ok {
		return updater.NotificationConfig{}, nil
	}

	var settings struct {
		NotificationConfig updater.NotificationConfig `json:"notification"`
	}

	if err := json.Unmarshal(jsonSettings, &settings); err != nil {
		return updater.NotificationConfig{}, fmt.Errorf("unmarshal json: %w", err)
	}

	return settings.NotificationConfig, nil
}

// SyncModuleRegistrySpec compares and updates current registry settings of a deployed module (in the ./openapi/values.yaml file)
// and the registry settings set in the related module source
func SyncModuleRegistrySpec(downloadedModulesDir, moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource) error {
	openAPIFile, err := os.Open(filepath.Join(downloadedModulesDir, moduleName, moduleVersion, "openapi/values.yaml"))
	if err != nil {
		return fmt.Errorf("open the '%s' module openapi values: %w", moduleName, err)
	}
	defer openAPIFile.Close()

	raw, err := io.ReadAll(openAPIFile)
	if err != nil {
		return fmt.Errorf("read from the '%s' module's openapi values: %w", moduleName, err)
	}

	var openAPISpec moduleOpenAPISpec
	if err = yaml.Unmarshal(raw, &openAPISpec); err != nil {
		return fmt.Errorf("unmarshal the '%s' module's registry spec: %w", moduleName, err)
	}

	registrySpec := openAPISpec.Properties.Registry.Properties

	dockercfg := reginjector.DockerCFGForModules(moduleSource.Spec.Registry.Repo, moduleSource.Spec.Registry.DockerCFG)

	if moduleSource.Spec.Registry.CA != registrySpec.CA.Default || dockercfg != registrySpec.DockerCFG.Default || moduleSource.Spec.Registry.Repo != registrySpec.Base.Default || moduleSource.Spec.Registry.Scheme != registrySpec.Scheme.Default {
		err = reginjector.InjectRegistryToModuleValues(filepath.Join(downloadedModulesDir, moduleName, moduleVersion), moduleSource)
	}

	return err
}

type moduleOpenAPISpec struct {
	Properties struct {
		Registry struct {
			Properties struct {
				Base struct {
					Default string `yaml:"default"`
				} `yaml:"base"`
				DockerCFG struct {
					Default string `yaml:"default"`
				} `yaml:"dockercfg"`
				Scheme struct {
					Default string `yaml:"default"`
				} `yaml:"scheme"`
				CA struct {
					Default string `yaml:"default"`
				} `yaml:"ca"`
			} `yaml:"properties"`
		} `yaml:"registry,omitempty"`
	} `yaml:"properties,omitempty"`
}
