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
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewKubernetesClient() (client.Client, error) {
	scheme := runtime.NewScheme()

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	restConfig := ctrl.GetConfigOrDie()
	opts := client.Options{
		Scheme: scheme,
	}

	k8sClient, err := client.New(restConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return k8sClient, nil
}

type DeckhouseService struct {
	dc dependency.Container

	registry        string
	registryOptions []cr.Option
}

func NewDeckhouseService(registryAddress string, registryConfig *RegistryConfig) *DeckhouseService {
	return &DeckhouseService{
		dc:              dependency.NewDependencyContainer(),
		registry:        registryAddress,
		registryOptions: GenerateRegistryOptions(registryConfig),
	}
}

func (svc *DeckhouseService) GetDeckhouseRelease(releaseChannel string) (*releaseMetadata, error) {
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

func (svc *DeckhouseService) ListDeckhouseReleases(ctx context.Context, fullList bool) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(svc.registry, svc.registryOptions...)
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

func (svc *DeckhouseService) GetModuleRelease(moduleName, releaseChannel string) (*moduleReleaseMetadata, error) {
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

func (svc *DeckhouseService) fetchModuleReleaseMetadata(img v1.Image) (*moduleReleaseMetadata, error) {
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

func (svc *DeckhouseService) fetchReleaseMetadata(image v1.Image) (*releaseMetadata, error) {
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
			fmt.Printf("layer is invalid: %s\n", err)
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
			fmt.Printf("Unmarshal CHANGELOG yaml failed: %s\n", err)
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

func (svc *DeckhouseService) fetchCooldown(image v1.Image) *metav1.Time {
	cfg, err := image.ConfigFile()
	if err != nil {
		fmt.Printf("image config error: %s\n", err)
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
			fmt.Printf("parse cooldown(%s) error: %s\n", v, err)
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
