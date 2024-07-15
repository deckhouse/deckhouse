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
	constraints         map[string]*semver.Constraints
}

func New(withBaseVersionLock bool) *Matcher {
	baseVersion, _ := semver.NewVersion("v1.0.0")
	return &Matcher{
		withBaseVersionLock: withBaseVersionLock,
		baseVersion:         baseVersion,
		constraints:         make(map[string]*semver.Constraints),
	}
}

func (m *Matcher) AddConstraint(name, rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return err
	}
	m.constraints[name] = constraint
	return nil
}

func (m *Matcher) ValidateByName(name string) error {
	constraint, ok := m.constraints[name]
	if !ok {
		return nil
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

func (m *Matcher) ValidateBaseVersion(baseVersion string) (string, error) {
	parsed, err := semver.NewVersion(baseVersion)
	if err != nil {
		return "", err
	}
	for name, constraint := range m.constraints {
		if _, errs := constraint.Validate(parsed); len(errs) != 0 {
			return name, errs[0]
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
