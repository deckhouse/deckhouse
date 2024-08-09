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
	"sync"

	"github.com/Masterminds/semver/v3"
)

type Matcher struct {
	withBaseVersionLock bool
	mtx                 sync.Mutex
	baseVersion         *semver.Version
	installed           map[string]*semver.Constraints
	release             map[string]*semver.Constraints
}

func New(withBaseVersionLock bool) *Matcher {
	baseVersion, _ := semver.NewVersion("v1.0.0")
	return &Matcher{
		withBaseVersionLock: withBaseVersionLock,
		baseVersion:         baseVersion,
		installed:           make(map[string]*semver.Constraints),
		release:             make(map[string]*semver.Constraints),
	}
}

func (m *Matcher) AddConstraint(name, rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return err
	}
	m.installed[name] = constraint
	return nil
}

func (m *Matcher) DeleteConstraints(name string) {
	delete(m.installed, name)
	delete(m.release, name)
}

func (m *Matcher) ValidateInstalled(name string) error {
	mod, ok := m.installed[name]
	if !ok {
		return nil
	}
	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}
	if _, errs := mod.Validate(m.baseVersion); len(errs) != 0 {
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
			// if there is a release constraint for the module which requirements are met try to validate it instead of installed
			if release, ok := m.release[module]; ok {
				if _, errs = release.Validate(parsed); len(errs) == 0 {
					return "", nil
				}
			}
			return module, errs[0]
		}
	}
	return "", nil
}

func (m *Matcher) ValidateRelease(name, rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return err
	}
	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}
	if _, errs := constraint.Validate(m.baseVersion); len(errs) != 0 {
		m.release[name] = constraint
		return errs[0]
	}
	// clear release constraint
	delete(m.release, name)
	return nil
}

func (m *Matcher) ChangeBaseVersion(version *semver.Version) {
	if m.withBaseVersionLock {
		m.mtx.Lock()
		defer m.mtx.Unlock()
	}
	m.baseVersion = version
}
