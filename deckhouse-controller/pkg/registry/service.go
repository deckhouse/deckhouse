// Copyright 2022 Flant JSC
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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/ettle/strcase"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Service struct {
	dc        dependency.Container
	k8sClient client.Client

	registry        string
	registryOptions []cr.Option
}

func NewService() *Service {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	restConfig := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		panic(fmt.Errorf("create kubernetes client: %w", err))
	}

	return &Service{
		dc:        dependency.NewDependencyContainer(),
		k8sClient: k8sClient,
	}
}

func (svc *Service) InitDeckhouseRegistry(ctx context.Context) error {
	secret := new(corev1.Secret)
	if err := svc.k8sClient.Get(ctx, types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-registry"}, secret); err != nil {
		return fmt.Errorf("list ModuleSource got an error: %w", err)
	}

	drs, err := ParseDeckhouseRegistrySecret(secret.Data)
	if err != nil {
		return fmt.Errorf("parse deckhouse registry secret: %w", err)
	}

	var discoverySecret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	if err := svc.k8sClient.Get(ctx, key, &discoverySecret); err != nil {
		return fmt.Errorf("get deckhouse discovery sectret got an error: %w", err)
	}

	clusterUUID, ok := discoverySecret.Data["clusterUUID"]
	if !ok {
		return fmt.Errorf("not found clusterUUID in discovery secret: %w", err)
	}

	ri := &RegistryInfo{
		DockerConfig: drs.DockerConfig,
		Scheme:       drs.Scheme,
		UserAgent:    string(clusterUUID),
	}

	svc.registry = drs.ImageRegistry
	svc.registryOptions = GenerateRegistryOptions(ri)

	return nil
}

func (svc *Service) InitModuleRegistry(ctx context.Context, moduleSource string) error {
	ms := new(v1alpha1.ModuleSource)
	if err := svc.k8sClient.Get(ctx, types.NamespacedName{Name: moduleSource}, ms); err != nil {
		return fmt.Errorf("get ModuleSource %s got an error: %w", moduleSource, err)
	}

	ri := &RegistryInfo{
		DockerConfig: ms.Spec.Registry.DockerCFG,
		Scheme:       ms.Spec.Registry.Scheme,
		CA:           ms.Spec.Registry.CA,
		UserAgent:    "deckhouse-controller/ModuleControllers",
	}

	svc.registry = ms.Spec.Registry.Repo
	svc.registryOptions = GenerateRegistryOptions(ri)

	return nil
}

func (svc *Service) ListModuleSource(ctx context.Context) ([]string, error) {
	msl := new(v1alpha1.ModuleSourceList)
	if err := svc.k8sClient.List(ctx, msl); err != nil {
		return nil, fmt.Errorf("list ModuleSource got an error: %w", err)
	}

	res := make([]string, 0, len(msl.Items))
	for _, ms := range msl.Items {
		res = append(res, ms.GetName())
	}

	return res, nil
}

func (svc *Service) ListDeckhouseReleases(ctx context.Context, fullList bool) ([]string, error) {
	tags, err := svc.ListModuleTags("", fullList)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	return tags, nil
}

var semVerRegex = regexp.MustCompile(`^v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?$`)

func (svc *Service) ListModules() ([]string, error) {
	// registry list modules <module-source>
	ls, err := svc.fetchListFromRegistry(svc.registry)
	if err != nil {
		return nil, err
	}

	return ls, err
}

func (svc *Service) ListModuleTags(moduleName string, fullList bool) ([]string, error) {
	// registry list module-release <module-source> <module-name>
	ls, err := svc.fetchListFromRegistry(svc.registry, moduleName)
	if err != nil {
		return nil, err
	}

	// if we need full tags list, not only semVer
	if fullList {
		return ls, nil
	}

	res := make([]string, 0, 1)
	for _, v := range ls {
		if semVerRegex.MatchString(v) {
			res = append(res, v)
		}
	}

	return res, err
}

func (svc *Service) fetchListFromRegistry(pathParts ...string) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(pathParts...), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %v", err)
	}

	ls, err := regCli.ListTags()
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %v", err)
	}

	return ls, nil
}

type canarySettings struct {
	Enabled  bool            `json:"enabled"`
	Waves    uint            `json:"waves"`
	Interval libapi.Duration `json:"interval"` // in minutes
}

type releaseMetadata struct {
	Version      string                    `json:"version"`
	Canary       map[string]canarySettings `json:"canary"`
	Requirements map[string]string         `json:"requirements"`
	Disruptions  map[string][]string       `json:"disruptions"`
	Suspend      bool                      `json:"suspend"`

	Changelog map[string]interface{}

	Cooldown *metav1.Time `json:"-"`
}

func (svc *Service) GetDeckhouseRelease(releaseChannel string) (*releaseMetadata, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, "release-channel"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %v", err)
	}

	return svc.fetchReleaseMetadata(img)
}

type moduleReleaseMetadata struct {
	Version *semver.Version `json:"version"`

	Changelog map[string]any
}

func (svc *Service) GetModuleRelease(moduleName, releaseChannel string) (*moduleReleaseMetadata, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName, "release"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %v", err)
	}

	moduleMetadata, err := svc.fetchModuleReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata error: %v", err)
	}

	if moduleMetadata.Version == nil {
		return nil, fmt.Errorf("module %q metadata malformed: no version found", moduleName)
	}

	return moduleMetadata, nil
}

func (svc *Service) fetchModuleReleaseMetadata(img v1.Image) (*moduleReleaseMetadata, error) {
	var meta = new(moduleReleaseMetadata)

	rc := mutate.Extract(img)
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err := rr.untarModuleMetadata(rc)
	if err != nil {
		return nil, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any
		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			meta.Changelog = make(map[string]any)
			return nil, nil
		}
		meta.Changelog = changelog
	}

	return meta, nil
}

func (svc *Service) fetchReleaseMetadata(image v1.Image) (*releaseMetadata, error) {
	var meta = new(releaseMetadata)

	layers, err := image.Layers()
	if err != nil {
		return nil, err
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("no layers found")
	}

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}
	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			fmt.Println("couldn't calculate layer size")
		}
		if size == 0 {
			// skip some empty werf layers
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			return nil, err
		}

		err = rr.untarDeckhouseLayer(rc)
		if err != nil {
			rc.Close()
			fmt.Println("layer is invalid: %s", err)
			continue
		}
		rc.Close()
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]interface{}
		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			fmt.Println("Unmarshal CHANGELOG yaml failed: %s", err)
			meta.Changelog = make(map[string]interface{})
			return meta, nil
		}
		meta.Changelog = changelog
	}

	cooldown := svc.fetchCooldown(image)
	if cooldown != nil {
		meta.Cooldown = cooldown
	}

	return meta, nil
}

func (svc *Service) fetchCooldown(image v1.Image) *metav1.Time {
	cfg, err := image.ConfigFile()
	if err != nil {
		fmt.Println("image config error: %s", err)
		return nil
	}

	if cfg == nil {
		return nil
	}

	if len(cfg.Config.Labels) == 0 {
		return nil
	}

	if v, ok := cfg.Config.Labels["cooldown"]; ok {
		t, err := parseTime(v)
		if err != nil {
			fmt.Println("parse cooldown(%s) error: %s", v, err)
			return nil
		}
		mt := metav1.NewTime(t)

		return &mt
	}

	return nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err == nil {
		return t, nil
	}

	t, err = time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t, nil
	}

	return time.Parse(time.RFC3339, s)
}
