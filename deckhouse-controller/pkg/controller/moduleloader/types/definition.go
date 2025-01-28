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
	"strconv"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	openapierrors "github.com/go-openapi/errors"
	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	DefinitionFile = "module.yaml"
)

type Definition struct {
	Name         string                 `yaml:"name"`
	Weight       uint32                 `yaml:"weight,omitempty"`
	Tags         []string               `yaml:"tags"`
	Stage        string                 `yaml:"stage"`
	Description  string                 `yaml:"description"`
	Requirements map[string]interface{} `json:"requirements"`

	DisableOptions DisableOptions `yaml:"disable"`

	Path string `yaml:"-"`
}

type DisableOptions struct {
	Confirmation bool   `yaml:"confirmation"`
	Message      string `yaml:"message"`
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
	var result, mErr *multierror.Error
	if errors.As(err, &mErr) {
		for _, me := range mErr.Errors {
			var e *openapierrors.Validation
			if errors.As(me, &e) {
				if e.Code() == 602 {
					continue
				}
			}
			result = multierror.Append(result, me)
		}
	}

	// now result will contain all validation errors, if any, except required.
	if result != nil {
		return fmt.Errorf("validate module: %w", result)
	}

	return nil
}

func (d *Definition) GetRequirements() map[string]string {
	requirements := make(map[string]string)
	if len(d.Requirements) == 0 {
		return requirements
	}

	for key, raw := range d.Requirements {
		if value, ok := raw.(string); ok {
			requirements[key] = value
		}
		if value, ok := raw.(bool); ok {
			requirements[key] = strconv.FormatBool(value)
		}
	}

	return requirements
}
