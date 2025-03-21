// Copyright 2024 Flant JSC
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

package types

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	openapierrors "github.com/go-openapi/errors"
	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	DefinitionFile = "module.yaml"
)

type Definition struct {
	Name         string                       `json:"name" yaml:"name"`
	Weight       uint32                       `json:"weight,omitempty" yaml:"weight,omitempty"`
	Tags         []string                     `json:"tags,omitempty" yaml:"tags,omitempty"`
	Subsystems   []string                     `json:"subsystems,omitempty" yaml:"subsystems,omitempty"`
	Namespace    string                       `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Stage        string                       `json:"stage,omitempty" yaml:"stage,omitempty"`
	Descriptions *ModuleDescriptions          `json:"descriptions,omitempty" yaml:"descriptions,omitempty"`
	Requirements *v1alpha1.ModuleRequirements `json:"requirements,omitempty" yaml:"requirements,omitempty"`

	DisableOptions *v1alpha1.ModuleDisableOptions `json:"disable,omitempty" yaml:"disable,omitempty"`

	Path string `yaml:"-"`
}

type ModuleDescriptions struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

func (d *Definition) Validate(values addonutils.Values, logger *log.Logger) error {
	if d.Weight < 900 || d.Weight > 999 {
		return errors.New("external module weight must be between 900 and 999")
	}

	if d.Path == "" {
		return errors.New("cannot validate module without path. Path is required to load openapi specs")
	}

	cb, vb, err := addonutils.ReadOpenAPIFiles(filepath.Join(d.Path, "openapi"))
	if err != nil {
		return fmt.Errorf("read open API files: %w", err)
	}

	dm, err := addonmodules.NewBasicModule(d.Name, d.Path, d.Weight, nil, cb, vb, addonmodules.WithLogger(logger.Named("basic-module")))
	if err != nil {
		return fmt.Errorf("new basic module: %w", err)
	}

	if values != nil {
		dm.SaveConfigValues(values)
	}

	err = dm.Validate()
	// next we will need to record all validation errors except required (602).
	var result error
	var mErr *multierror.Error
	if errors.As(err, &mErr) {
		for _, me := range mErr.Errors {
			var e *openapierrors.Validation

			if errors.As(me, &e) {
				if e.Code() == 602 {
					continue
				}
			}

			result = errors.Join(result, me)
		}
	}

	// now result will contain all validation errors, if any, except required.
	if result != nil {
		return fmt.Errorf("validate module: %w", result)
	}

	return nil
}

func (d *Definition) Annotations() map[string]string {
	annotations := make(map[string]string)

	if d.Descriptions != nil {
		if len(d.Descriptions.Ru) > 0 {
			annotations[v1alpha1.ModuleAnnotationDescriptionRu] = d.Descriptions.Ru
		}
		if len(d.Descriptions.En) > 0 {
			annotations[v1alpha1.ModuleAnnotationDescriptionEn] = d.Descriptions.En
		}
	}

	return annotations
}

func (d *Definition) Labels() map[string]string {
	labels := make(map[string]string)

	if strings.HasPrefix(d.Name, "cni-") {
		labels["module.deckhouse.io/cni"] = ""
	}

	if strings.HasPrefix(d.Name, "cloud-provider-") {
		labels["module.deckhouse.io/cloud-provider"] = ""
	}

	if len(d.Tags) != 0 {
		for _, tag := range d.Tags {
			labels["module.deckhouse.io/"+tag] = ""
		}
	}

	return labels
}
