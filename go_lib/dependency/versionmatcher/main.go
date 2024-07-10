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
	"github.com/Masterminds/semver/v3"
)

type Matcher struct {
	baseVersion *semver.Version
	constraints map[string]*semver.Constraints
}

func New() *Matcher {
	baseVersion, _ := semver.NewVersion("0.0.0")
	return &Matcher{baseVersion: baseVersion, constraints: make(map[string]*semver.Constraints)}
}

func (m *Matcher) SetBaseVersion(baseVersion *semver.Version) {
	m.baseVersion = baseVersion
}

func (m *Matcher) AddConstraint(name, rawConstraint string) error {
	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return err
	}
	m.constraints[name] = constraint
	return nil
}

func (m *Matcher) Validate(name string) error {
	constraint, ok := m.constraints[name]
	if !ok {
		return nil
	}
	if _, errs := constraint.Validate(m.baseVersion); len(errs) != 0 {
		return errs[0]
	}
	return nil
}
