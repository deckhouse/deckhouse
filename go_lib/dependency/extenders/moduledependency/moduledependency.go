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

package moduledependency

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"github.com/hashicorp/go-multierror"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name extenders.ExtenderName = "ModuleDependency"
)

var (
	instance *Extender
	once     sync.Once
)

var (
	_ extenders.Extender            = &Extender{}
	_ extenders.TopologicalExtender = &Extender{}
	_ extenders.StatefulExtender    = &Extender{}
)

type Extender struct {
	modulesVersionHelper func(moduleName string) (string, error)
	modulesStateHelper   func() []string
	modules              map[string]*versionmatcher.Matcher
	logger               *log.Logger
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:  log.Default().With("extender", Name),
			modules: make(map[string]*versionmatcher.Matcher),
		}
	})
	return instance
}

func (e *Extender) constraintFormsLoop(name string, value map[string]string) (bool, string) {
	itinerary := make([]string, 0, len(value))
	for constraint := range value {
		if name != constraint {
			itinerary = append(itinerary, constraint)
		}
	}

	for len(itinerary) > 0 {
		parentModule := itinerary[0]
		itinerary = itinerary[1:]
		if constraint, found := e.modules[parentModule]; found {
			for _, parentModuleConstraintName := range constraint.GetConstraintsNames() {
				if parentModuleConstraintName == name {
					return true, parentModule
				}

				itinerary = append(itinerary, parentModuleConstraintName)
			}
		}
	}

	return false, ""
}

func (e *Extender) SetModulesVersionHelper(f func(moduleName string) (string, error)) {
	e.modulesVersionHelper = f
}

func (e *Extender) createConstrainstForModule(name string, value map[string]string) (*versionmatcher.Matcher, error) {
	constraints := versionmatcher.New(false)
	for dependency, constraint := range value {
		if name == dependency {
			e.logger.Warn(fmt.Sprintf("parent module '%s' is excluded from the '%s' module constraints", dependency, name))
			continue
		}

		if err := constraints.AddConstraint(dependency, constraint); err != nil {
			return nil, err
		}
	}

	return constraints, nil
}

func (e *Extender) AddConstraint(name string, value map[string]string) error {
	constraints, err := e.createConstrainstForModule(name, value)
	if err != nil {
		return err
	}

	e.modules[name] = constraints
	e.logger.Debugf("installed constraint for the '%s' module is added", name)

	return nil
}

func errorFormatter(es []error) string {
	if len(es) == 1 {
		return fmt.Sprintf("1 error occurred: %s", es[0])
	}

	errors := make([]string, 0, len(es))
	for _, err := range es {
		errors = append(errors, fmt.Sprintf("%s", err))
	}
	slices.Sort(errors)

	return fmt.Sprintf("%d errors occurred: %s", len(es), strings.Join(errors, "; "))
}

// removePrereleaseAndMetadata returns a version without prerelease and metadata parts
func removePrereleaseAndMetadata(version *semver.Version) (*semver.Version, error) {
	if len(version.Prerelease()) > 0 {
		woPrerelease, err := version.SetPrerelease("")
		if err != nil {
			return nil, err
		}

		version = &woPrerelease
	}

	if len(version.Metadata()) > 0 {
		woMetadata, err := version.SetMetadata("")
		if err != nil {
			return nil, err
		}

		version = &woMetadata
	}

	return version, nil
}

// parseParentVersion parses a string representing semver.Version and returns a release version without prerelease and meta info
// because mastermind semver package doesn't do its job well when comparing versions with prerelease
func parseParentVersion(parentVersion string) (*semver.Version, error) {
	parsedParentVersion, err := semver.NewVersion(parentVersion)
	if err != nil {
		return nil, err
	}

	return removePrereleaseAndMetadata(parsedParentVersion)
}

func (e *Extender) ValidateRelease(moduleName, moduleRelease string, version *semver.Version, value map[string]string) error {
	validateErr := &multierror.Error{ErrorFormat: errorFormatter}
	// check if the new constraints may impose a loop
	if formsLoop, dependentModule := e.constraintFormsLoop(moduleName, value); formsLoop {
		validateErr = multierror.Append(validateErr, fmt.Errorf("module depency error: adding \"%s\" module release dependencies forms a dependency loop with the installed \"%s\" module", moduleName, dependentModule))
		return validateErr
	}

	constraints, err := e.createConstrainstForModule(moduleName, value)
	if err != nil {
		validateErr = multierror.Append(validateErr, fmt.Errorf("could not validate the \"%s\" module dependencies: %s", moduleName, err.Error()))
		return validateErr
	}

	// check if the new requirements are satisfied
	for _, parentModule := range constraints.GetConstraintsNames() {
		parentVersion, err := e.modulesVersionHelper(parentModule)
		if err != nil {
			validateErr = multierror.Append(validateErr, fmt.Errorf("could not get the \"%s\" module version: %s", parentModule, err.Error()))
			if apierrors.IsNotFound(err) {
				continue
			}
		}

		parsedParentVersion, err := parseParentVersion(parentVersion)
		if err != nil {
			validateErr = multierror.Append(validateErr, fmt.Errorf("the \"%s\" module dependency \"%s\" has unparsable version", moduleName, parentModule))
			continue
		}

		if err := constraints.ValidateModuleVersion(parentModule, parsedParentVersion); err != nil {
			validateErr = multierror.Append(validateErr, fmt.Errorf("the \"%s\" module dependency \"%s\" does not meet the version constraint: %s", moduleName, parentModule, err.Error()))
		}
	}

	sanitizedVersion, err := removePrereleaseAndMetadata(version)
	if err != nil {
		validateErr = multierror.Append(validateErr, fmt.Errorf("couldn't get the \"%s\" module version without prerelease and metadata info: %s", moduleName, err.Error()))
	}

	// check if the new module's version breaks current constraints
	for dependentModule, constraints := range e.modules {
		if err := constraints.ValidateModuleVersion(moduleName, sanitizedVersion); err != nil {
			validateErr = multierror.Append(validateErr, fmt.Errorf("the \"%s\" module dependency \"%s\" does not meet the version constraint if the \"%s\" module release is installed: %s", dependentModule, moduleName, moduleRelease, err.Error()))
		}
	}

	return validateErr.ErrorOrNil()
}

func (e *Extender) DeleteConstraint(name string) {
	delete(e.modules, name)
}

// Name implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool {
	return true
}

// GetTopologicalHints implements TopologicalExtender interface of the addon-operator
func (e *Extender) GetTopologicalHints(moduleName string) []string {
	hints := make([]string, 0)
	if constraints, found := e.modules[moduleName]; found {
		hints = append(hints, constraints.GetConstraintsNames()...)
	}

	return hints
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(moduleName string, _ map[string]string) (*bool, error) {
	constraints, found := e.modules[moduleName]
	if !found {
		return nil, nil
	}

	err := &multierror.Error{ErrorFormat: errorFormatter}
	enabledModules := e.modulesStateHelper()

	for _, parentModule := range constraints.GetConstraintsNames() {
		exists := true
		parentVersion, getErr := e.modulesVersionHelper(parentModule)
		if getErr != nil {
			if !apierrors.IsNotFound(getErr) {
				return nil, &scherror.PermanentError{Err: fmt.Errorf("could not get the \"%s\" module version: %s", parentModule, getErr)}
			}
			exists = false
		}

		// check if the parent module is disabled/absent
		if !slices.Contains(enabledModules, parentModule) {
			msg := "not found"
			if exists {
				msg = "is disabled"
			}
			err = multierror.Append(err, fmt.Errorf("the \"%s\" module dependency \"%s\" %s", moduleName, parentModule, msg))
			continue
		}

		parsedParentVersion, parseErr := parseParentVersion(parentVersion)
		if parseErr != nil {
			err = multierror.Append(err, fmt.Errorf("the \"%s\" module dependency \"%s\" has unparsable version", moduleName, parentModule))
			continue
		}

		// check if the parent module is of an inappropriate version
		if versionErr := constraints.ValidateModuleVersion(parentModule, parsedParentVersion); versionErr != nil {
			err = multierror.Append(err, fmt.Errorf("the \"%s\" module dependency \"%s\" does not meet the version constraint: %s", moduleName, parentModule, versionErr.Error()))
		}
	}

	if err.ErrorOrNil() != nil {
		return ptr.To(false), err
	}

	return ptr.To(true), nil
}

// SetModulesStateHelper implements StatefulExtender interface of the addon-operator
func (e *Extender) SetModulesStateHelper(f func() []string) {
	e.modulesStateHelper = f
}
