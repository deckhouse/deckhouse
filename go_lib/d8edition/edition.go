// Copyright 2025 Flant JSC
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

package d8edition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/confighandler"
)

const (
	path = "/deckhouse/editions"

	deckhouseConfig = "deckhouse"

	Embedded = "embedded"
)

type Edition struct {
	Name    string                       `yaml:"-"`
	Bundle  string                       `yaml:"-"`
	Modules map[string]map[string]Module `json:"modules" yaml:"modules"`
}

type Config struct {
	Bundle  string `json:"bundle"`
	Edition string `json:"edition"`
}

type Module struct {
	Available bool  `json:"available" yaml:"available"`
	Enabled   *bool `json:"enabled" yaml:"enabled,omitempty"`
}

func Parse(ctx context.Context, cli client.Client) (*Edition, error) {
	config := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: deckhouseConfig}, config); err != nil {
		return nil, fmt.Errorf("get the deckhouse module config: %w", err)
	}

	values, err := confighandler.ValuesByModuleConfig(config)
	if err != nil {
		return nil, fmt.Errorf("get values: %w", err)
	}

	raw, _ := values.AsBytes("yaml")

	parsed := new(Config)
	if err = yaml.Unmarshal(raw, parsed); err != nil {
		return nil, fmt.Errorf("unmarshal deckhouse config: %w", err)
	}

	if parsed.Bundle == "" || parsed.Edition == "" {
		parsed.Bundle = "default"
		parsed.Edition = "FE"
	}

	edition := new(Edition)
	edition.Name = parsed.Edition
	edition.Bundle = parsed.Bundle

	parsed.Bundle = strings.ToLower(strings.TrimSpace(parsed.Bundle))
	parsed.Edition = strings.ToLower(strings.TrimSpace(parsed.Edition)) + ".yaml"

	content, err := os.ReadFile(filepath.Join(path, parsed.Bundle, parsed.Edition))
	if err != nil {
		return nil, fmt.Errorf("read the '%s/%s' edition file: %w", parsed.Bundle, parsed.Edition, err)
	}

	if err = yaml.Unmarshal(content, edition); err != nil {
		return nil, fmt.Errorf("unmarshal the '%s/%s' edition file: %w", parsed.Bundle, parsed.Edition, err)
	}

	return edition, nil
}

func (e *Edition) String() string {
	return fmt.Sprintf("%s/%s", e.Bundle, e.Name)
}

func (e *Edition) IsAvailable(sourceName, moduleName string) *bool {
	if source, ok := e.Modules[sourceName]; ok {
		if module, ok := source[moduleName]; ok {
			return &module.Available
		} else {
			return ptr.To(false)
		}
	}
	return nil
}

func (e *Edition) SyncModules(ctx context.Context, cli client.Client) error {
	modules := new(v1alpha1.ModuleList)
	if err := cli.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for _, module := range modules.Items {
		if err := e.syncModule(ctx, cli, &module); err != nil {
			return fmt.Errorf("sync '%s' module: %w", module.Name, err)
		}
	}

	return nil
}

func (e *Edition) syncModule(ctx context.Context, cli client.Client, module *v1alpha1.Module) error {
	var editionModule *Module
	var availableSource string
	if module.Properties.Source == "" {
		for _, available := range module.Properties.AvailableSources {
			if source, ok := e.Modules[available]; ok {
				if mod, ok := source[module.Name]; ok {
					editionModule = &mod
					availableSource = available
					break
				}
			}
		}
	} else {
		if source, ok := e.Modules[strings.ToLower(module.Properties.Source)]; ok {
			if mod, ok := source[module.Name]; ok {
				editionModule = &mod
			}
		}
	}
	if editionModule == nil || editionModule.Enabled == nil {
		// the embedded module is not present in the edition, it should be removed
		if editionModule == nil && module.IsEmbedded() {
			return cli.Delete(ctx, module)
		}
		return nil
	}

	if module.Name == deckhouseConfig {
		return nil
	}

	config := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: module.Name}, config); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the module config: %w", err)
		}
		config = &v1alpha1.ModuleConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       v1alpha1.ModuleConfigKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: module.Name,
			},
			Spec: v1alpha1.ModuleConfigSpec{
				Source:  availableSource,
				Enabled: editionModule.Enabled,
			},
		}
		return cli.Create(ctx, config)
	}

	if *config.Spec.Enabled == *editionModule.Enabled {
		return nil
	}

	for _, field := range config.ManagedFields {
		if field.Subresource == "" && field.Manager != "deckhouse-controller" {
			return nil
		}
	}

	config.Spec.Enabled = editionModule.Enabled

	return cli.Update(ctx, config)
}
