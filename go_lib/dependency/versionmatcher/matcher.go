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

package versionmatcher

import (
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
)

type Matcher struct {
	withBaseVersionLock bool
	mtx                 sync.Mutex
	baseVersion         *semver.Version
	installed           map[string]*semver.Constraints
}

func New(withBaseVersionLock bool) *Matcher {
	baseVersion, _ := semver.NewVersion("v2.0.0")

	return &Matcher{
		withBaseVersionLock: withBaseVersionLock,
		baseVersion:         baseVersion,
		installed:           make(map[string]*semver.Constraints),
	}
}

func (m *Matcher) GetBaseVersion() *semver.Version {
	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}

	return m.baseVersion
}

func (m *Matcher) AddConstraint(name, rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return err
	}

	m.installed[name] = constraint

	return nil
}

func (m *Matcher) DeleteConstraint(name string) {
	delete(m.installed, name)
}

func (m *Matcher) Has(name string) bool {
	_, ok := m.installed[name]

	return ok
}

func (m *Matcher) GetConstraintsNames() []string {
	names := make([]string, 0, len(m.installed))
	for name := range m.installed {
		names = append(names, name)
	}

	return names
}

func (m *Matcher) Validate(name string) error {
	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}

	if _, errs := m.installed[name].Validate(m.baseVersion); len(errs) != 0 {
		return errs[0]
	}

	return nil
}

func (m *Matcher) ValidateBaseVersion(baseVersion string) (string, error) {
	parsed, err := semver.NewVersion(baseVersion)
	if err != nil {
		return "", err
	}

	for module, installed := range m.installed {
		if _, errs := installed.Validate(parsed); len(errs) != 0 {
			return module, errs[0]
		}
	}

	return "", nil
}

func (m *Matcher) ValidateConstraint(rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return err
	}

	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}

	if _, errs := constraint.Validate(m.baseVersion); len(errs) != 0 {
		return errs[0]
	}

	return nil
}

func (m *Matcher) ChangeBaseVersion(version *semver.Version) {
	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}

	m.baseVersion = version
}

// ValidateModuleVersions ignores prerelease/metadata part when comparing versions
func (m *Matcher) ValidateModuleVersion(name string, version *semver.Version) error {
	constraint, found := m.installed[name]
	if !found {
		return nil
	}

	if !constraint.Check(version) {
		return fmt.Errorf("the '%s' version does not satisfy the '%s' constraint", version.Original(), constraint.String())
	}

	return nil
}
