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
	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"
)

const (
	Name extenders.ExtenderName = "DeckhouseVersion"
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	logger         logger.Logger
	currentVersion *semver.Version
	constraints    map[string]*semver.Constraints
}

func New() (*Extender, error) {
	version := semver.MustParse("v0.0.0")
	if raw, err := os.ReadFile("/deckhouse/version"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		if parsed, err := semver.NewVersion(string(raw)); err == nil {
			version = parsed
		}
	}
	return &Extender{
		currentVersion: version,
		constraints:    make(map[string]*semver.Constraints),
		logger:         log.WithField("extender", Name),
	}, nil
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		e.logger.Errorf("adding deckhouseVersion constraint for %q failed: %v", name, err)
		return err
	}
	e.logger.Debugf("constraint for %q is added", name)
	e.constraints[name] = constraint
	return nil
}

func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	constraint, ok := e.constraints[name]
	if !ok {
		return nil, nil
	}
	if _, errs := constraint.Validate(e.currentVersion); len(errs) != 0 {
		e.logger.Errorf("requirements of %s are not satisfied: current deckhouse version is not suitable: %s", name, errs[0].Error())
		return pointer.Bool(false), nil
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
}

func (e *Extender) IsTerminator() {

}

func NewError(moduleName string) error {
	return fmt.Errorf("requirements of %s are not satisfied: current deckhouse version is not suitable", moduleName)
}
