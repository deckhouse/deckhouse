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

package utils //nolint:revive

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/gofrs/uuid/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// GenerateRegistryOptionsFromModuleSource fetches settings from ModuleSource and generate registry options from them
func GenerateRegistryOptionsFromModuleSource(ms *v1alpha1.ModuleSource, clusterUUID string, logger *log.Logger) []cr.Option {
	rconf := &RegistryConfig{
		DockerConfig: ms.Spec.Registry.DockerCFG,
		Scheme:       ms.Spec.Registry.Scheme,
		CA:           ms.Spec.Registry.CA,
		UserAgent:    clusterUUID,
	}

	return GenerateRegistryOptions(rconf.ForRepository(ms.Spec.Registry.Repo, logger), logger)
}

// agentCAFile is where the node agent's authority is read from. A variable rather than the
// constant itself only so that a test can point it at a file it is allowed to create.
var agentCAFile = registry_const.AgentCAFile

// Dial is the address to actually connect to in order to reach a repository.
//
// Everything the cluster records names the in-cluster registry, because a recorded address is
// read by more than one party: it ends up in pod image references, in a ModuleSource, in a
// status. The loopback address belongs in none of those — it is node-local, and a cluster-wide
// object naming it is wrong even where it happens to work. What made that concrete was a
// ModuleSource whose repository named the loopback: the pods kept running on content their
// nodes already held, while the agent answered 502 to every fresh pull, since a request naming
// the agent as its own registry is a loop and is refused.
//
// So the translation happens here, at the one place that has to dial, and nowhere else.
func Dial(repository string) string {
	if host, rest, found := strings.Cut(repository, "/"); found && host == registry_const.Host {
		return registry_const.ProxyHost + "/" + rest
	}
	if repository == registry_const.Host {
		return registry_const.ProxyHost
	}
	return repository
}

// ForRepository adjusts a registry client configuration for the repository it will fetch
// from, and does nothing at all unless that repository is served by the node agent.
//
// The agent is what the registry module puts in front of every pull once it manages the pull
// path, and a process — unlike a container runtime, which is handed a drop-in that redirects
// it — has to dial the agent to fetch through it. Two things about the agent are not
// properties of the registry the cluster was told about, and cannot be:
//
//   - It serves HTTPS, whatever the upstream behind it speaks. An upstream reached over plain
//     HTTP is perfectly ordinary, and taking the configured scheme here would send a plaintext
//     request to a TLS listener.
//   - Its authority is generated on the node at bootstrap and never leaves it, so no cluster
//     object can carry it — every node has a different one. That is deliberate: a node's
//     ability to pull must not wait for cluster-wide certificate material to arrive. It is why
//     this pod mounts the authority from the host instead.
//
// Credentials are left as they are, and are simply not used: the agent authenticates to the
// registry behind it with credentials of its own, and a docker config naming other hosts has
// nothing to say about the loopback address.
func (rc *RegistryConfig) ForRepository(repository string, logger *log.Logger) *RegistryConfig {
	// Either spelling of the same agent: as recorded, or as dialled.
	if !registry_const.IsInCluster(repository) && !registry_const.IsLocalAgent(repository) {
		return rc
	}

	rc.Scheme = registry_const.Scheme

	authority, err := os.ReadFile(agentCAFile)
	if err != nil {
		// Said out loud, because the failure it leads to says nothing about a file: the
		// fetch fails to verify a certificate, and the reason is that the authority which
		// signed it was never read.
		logger.Error("cannot read the registry agent authority, so nothing fetched through "+
			"the agent on this node can be verified",
			slog.String("path", registry_const.AgentCAFile),
			log.Err(err))
		return rc
	}
	rc.CA = string(authority)

	return rc
}

// RegistryConfig is what a client for the registry this secret describes is built from.
func (s *DeckhouseRegistrySecret) RegistryConfig(userAgent string, logger *log.Logger) *RegistryConfig {
	rc := &RegistryConfig{
		DockerConfig: s.DockerConfig,
		Scheme:       s.Scheme,
		CA:           s.CA,
		UserAgent:    userAgent,
	}

	return rc.ForRepository(s.Fetch(), logger)
}

// Fetch is the repository this process pulls from, ready to be dialled.
func (s *DeckhouseRegistrySecret) Fetch() string {
	if s.FetchRegistry != "" {
		return Dial(s.FetchRegistry)
	}
	return Dial(s.ImageRegistry)
}

type RegistryConfig struct {
	DockerConfig string
	Login        string
	Password     string
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
		cr.WithDockerCfgAuth(ri.DockerConfig),
		cr.WithUserPasswordAuth(ri.Login, ri.Password),
		cr.WithUserAgent(ri.UserAgent),
		cr.WithCA(ri.CA),
		cr.WithInsecureSchema(strings.ToLower(ri.Scheme) == "http"),
	}

	return opts
}

type DeckhouseRegistrySecret struct {
	DockerConfig          string
	Address               string
	ClusterIsBootstrapped bool
	ImageRegistry         string
	Path                  string
	Scheme                string
	CA                    string

	// FetchRegistry is where THIS process fetches from, which is not always the registry
	// ImageRegistry names.
	//
	// Its own field because the two have different readers. ImageRegistry is the registry as
	// seen from outside the cluster — dhctl reads it from wherever it is run and will not
	// touch a cluster whose docker config has no credentials for the host it names — while
	// this one is read only in the cluster, where the registry module may have put a node
	// agent in front of every pull. Going through that agent is what makes a change of
	// registry reach this process: nothing writes a registry address into the secret when the
	// module moves the pull path.
	//
	// Empty on a cluster whose secret predates the field, and then ImageRegistry is used.
	FetchRegistry string
}

var ErrDockerConfigFieldIsNotFound = errors.New("secret has no .dockerconfigjson field")
var ErrAddressFieldIsNotFound = errors.New("secret has no address field")
var ErrClusterIsBootstrappedFieldIsNotFound = errors.New("secret has no clusterIsBootstrapped field")
var ErrImageRegistryFieldIsNotFound = errors.New("secret has no imagesRegistry field")
var ErrPathFieldIsNotFound = errors.New("secret has no path field")
var ErrSchemeFieldIsNotFound = errors.New("secret has no scheme field")
var ErrCAFieldIsNotFound = errors.New("secret has no ca field")

func ParseDeckhouseRegistrySecret(data map[string][]byte) (*DeckhouseRegistrySecret, error) {
	var err error

	dockerConfig, ok := data[".dockerconfigjson"]
	if !ok {
		err = errors.Join(err, ErrDockerConfigFieldIsNotFound)
	}

	address, ok := data["address"]
	if !ok {
		err = errors.Join(err, ErrAddressFieldIsNotFound)
	}

	clusterIsBootstrappedRaw, ok := data["clusterIsBootstrapped"]
	if !ok {
		err = errors.Join(err, ErrClusterIsBootstrappedFieldIsNotFound)
	}

	var clusterIsBootstrapped bool
	var castErr error
	if ok {
		trimmedBool := strings.ReplaceAll(string(clusterIsBootstrappedRaw), "\"", "")

		clusterIsBootstrapped, castErr = strconv.ParseBool(trimmedBool)
		if castErr != nil {
			err = errors.Join(err, fmt.Errorf("clusterIsBootstrapped is not bool: %w", castErr))
		}
	}

	imagesRegistry, ok := data["imagesRegistry"]
	if !ok {
		err = errors.Join(err, ErrImageRegistryFieldIsNotFound)
	}

	path, ok := data["path"]
	if !ok {
		err = errors.Join(err, ErrPathFieldIsNotFound)
	}

	scheme, ok := data["scheme"]
	if !ok {
		err = errors.Join(err, ErrSchemeFieldIsNotFound)
	}

	ca, ok := data["ca"]
	if !ok {
		err = errors.Join(err, ErrCAFieldIsNotFound)
	}

	// Optional, deliberately: every cluster installed before it existed has a secret without
	// it, and making it required would refuse to read any of them.
	fetchRegistry := data["fetchRegistry"]

	return &DeckhouseRegistrySecret{
		DockerConfig:          string(dockerConfig),
		Address:               string(address),
		ClusterIsBootstrapped: clusterIsBootstrapped,
		ImageRegistry:         string(imagesRegistry),
		FetchRegistry:         string(fetchRegistry),
		Path:                  string(path),
		Scheme:                string(scheme),
		CA:                    string(ca),
	}, err
}

// Update updates object with retryOnConflict to avoid conflict
func Update[Object client.Object](ctx context.Context, cli client.Client, object Object, updater func(obj Object) bool) error {
	err := retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := cli.Get(ctx, client.ObjectKey{Name: object.GetName()}, object); err != nil {
				return fmt.Errorf("get: %w", err)
			}
			if updater(object) {
				return cli.Update(ctx, object)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("on error: %w", err)
	}
	return nil
}

// UpdateStatus updates object status with retryOnConflict to avoid conflict
func UpdateStatus[Object client.Object](ctx context.Context, cli client.Client, object Object, updater func(obj Object) bool) error {
	err := retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := cli.Get(ctx, client.ObjectKey{Name: object.GetName()}, object); err != nil {
				return fmt.Errorf("get: %w", err)
			}
			if updater(object) {
				return cli.Status().Update(ctx, object)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("on error: %w", err)
	}
	return nil
}

// GetUpdatePolicyByModule returns policy for the module, if no policy, embeddedPolicy is returned
func GetUpdatePolicyByModule(ctx context.Context, cli client.Client, embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer, moduleName string) (*v1alpha2.ModuleUpdatePolicy, error) {
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
			return false, fmt.Errorf("get: %w", err)
		}
		return false, nil
	}
	return true, nil
}

// GetClusterUUID gets uuid from the secret or generate a new one
func GetClusterUUID(ctx context.Context, cli client.Client) string {
	// attempt to read the cluster UUID from a secret
	secret := new(corev1.Secret)
	if err := cli.Get(ctx, client.ObjectKey{Namespace: app.NamespaceDeckhouse, Name: app.SecretDiscovery}, secret); err != nil {
		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	// generate a random UUID if the key is missing
	return uuid.Must(uuid.NewV4()).String()
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

	// Check if path needs to be migrated from old format (e.g., "/module/v1.0.0" or "/module/dev")
	// to new format ("/modules/module")
	needsPathUpdate := !strings.HasPrefix(md.Spec.Path, "/modules/")

	if md.Spec.Version != moduleVersion || md.Spec.Checksum != moduleChecksum || needsPathUpdate {
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

// DeleteModuleDocumentation deletes module documentation, ignoring NotFound
func DeleteModuleDocumentation(ctx context.Context, cli client.Client, moduleName string) error {
	md := &v1alpha1.ModuleDocumentation{
		ObjectMeta: metav1.ObjectMeta{
			Name: moduleName,
		},
	}

	if err := cli.Delete(ctx, md); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the '%s' module documentation: %w", moduleName, err)
	}

	return nil
}

// EnsureModuleDocumentationForRelease ensures module documentation from a deployed release
func EnsureModuleDocumentationForRelease(ctx context.Context, cli client.Client, release *v1alpha1.ModuleRelease) error {
	// mount point path: /modules/<module>
	modulePath := fmt.Sprintf("/modules/%s", release.GetModuleName())
	moduleVersion := "v" + release.GetVersion().String()

	moduleChecksum := release.Labels[v1alpha1.ModuleReleaseLabelReleaseChecksum]
	if moduleChecksum == "" {
		moduleChecksum = fmt.Sprintf("%x", md5.Sum([]byte(moduleVersion)))
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: v1alpha1.ModuleReleaseGVK.GroupVersion().String(),
		Kind:       v1alpha1.ModuleReleaseGVK.Kind,
		Name:       release.GetName(),
		UID:        release.GetUID(),
		Controller: ptr.To(true),
	}

	return EnsureModuleDocumentation(ctx, cli, release.GetModuleName(), release.GetModuleSource(), moduleChecksum, moduleVersion, modulePath, ownerRef)
}

// GetNotificationConfig gets config from discovery secret
func GetNotificationConfig(ctx context.Context, cli client.Client) (releaseUpdater.NotificationConfig, error) {
	secret := new(corev1.Secret)
	if err := cli.Get(ctx, client.ObjectKey{Name: app.SecretDiscovery, Namespace: app.NamespaceDeckhouse}, secret); err != nil {
		return releaseUpdater.NotificationConfig{}, fmt.Errorf("get secret: %w", err)
	}

	// TODO: remove this dependency
	jsonSettings, ok := secret.Data["updateSettings.json"]
	if !ok {
		return releaseUpdater.NotificationConfig{}, nil
	}

	var settings struct {
		NotificationConfig releaseUpdater.NotificationConfig `json:"notification"`
	}

	if err := json.Unmarshal(jsonSettings, &settings); err != nil {
		return releaseUpdater.NotificationConfig{}, fmt.Errorf("unmarshal json: %w", err)
	}

	return settings.NotificationConfig, nil
}

func BuildRegistryValue(source *v1alpha1.ModuleSource) *addonmodules.Registry {
	if source == nil {
		return nil
	}

	reg := new(addonmodules.Registry)

	reg.Base = source.Spec.Registry.Repo
	reg.DockerCfg = buildDockerCfg(reg.Base, source.Spec.Registry.DockerCFG)
	reg.Scheme = source.Spec.Registry.Scheme
	reg.CA = source.Spec.Registry.CA

	return reg
}

// buildDockerCfg
// according to the deckhouse docs, for anonymous registry access we must have the value:
// {"auths": { "<PROXY_REGISTRY>": {}}}
// but it could be empty for a ModuleSource.
// modules are not ready to catch empty string, so we have to fill it with the default value
func buildDockerCfg(repo, dockercfg string) string {
	if len(dockercfg) != 0 {
		return dockercfg
	}

	index := strings.Index(repo, "/")
	var registry string
	if index != -1 {
		registry = repo[:index]
	} else {
		registry = repo
	}

	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"auths": {"%s": {}}}`, registry)))
}
