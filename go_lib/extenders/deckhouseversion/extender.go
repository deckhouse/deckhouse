/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deckhouseversion

import (
	"errors"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	"k8s.io/utils/pointer"
)

const (
	Name extenders.ExtenderName = "DeckhouseVersion"
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	currentVersion     *semver.Version
	modulesConstraints map[string]*semver.Constraints
}

func New() (*Extender, error) {
	version := semver.MustParse("v0.0.0")
	if raw, err := os.ReadFile("/deckouse/version"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		if parsed, err := semver.NewVersion(string(raw)); err == nil {
			version = parsed
		}
	}
	return &Extender{currentVersion: version, modulesConstraints: make(map[string]*semver.Constraints)}, nil
}

func (e *Extender) AddConstraint(moduleName, moduleDefConstraint string) error {
	constraint, err := semver.NewConstraint(moduleDefConstraint)
	if err != nil {
		return err
	}
	e.modulesConstraints[moduleName] = constraint
	return nil
}

func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

func (e *Extender) Filter(moduleName string, _ map[string]string) (*bool, error) {
	constraint, ok := e.modulesConstraints[moduleName]
	if !ok {
		return nil, nil
	}
	valid, errs := constraint.Validate(e.currentVersion)
	if len(errs) != 0 {
		return pointer.Bool(false), errs[0]
	}
	return pointer.Bool(valid), nil
}

func NewError(moduleName string) error {
	return fmt.Errorf("requirements of module %s are not satisfied: current deckhouse version is not suitable", moduleName)
}
