// Copyright 2023 Flant JSC
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

package versions

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

var (
	ErrNoVersion = errors.New("no such version")
)

func parse(v string) (*semver.Version, error) {
	return semver.NewVersion(v)
}

func parseFromInt(major, minor, patch uint64) *semver.Version {
	version := fmt.Sprintf("v%d.%d", major, minor)
	if patch > 0 {
		version = version + "." + strconv.FormatUint(patch, 10)
	}
	v, _ := parse(version)
	return v
}

type latestVersions map[semver.Version]semver.Version /* map[<maj>.<min>]max(<maj>.<min>.<patch>) */

func (vs latestVersions) SetString(v string) (bool, error) {
	versionWithPatch, err := parse(v)
	if err != nil {
		return false, err
	}
	return vs.Set(*versionWithPatch)
}

func (vs latestVersions) Set(new semver.Version) (bool, error) {
	old, err := vs.Get(new)
	switch {
	case errors.Is(err, ErrNoVersion):
	case err != nil:
		return false, err
	case old.GreaterThan(&new):
		return false, nil
	}

	vs[prepareKey(new)] = new
	return true, nil
}

func (vs latestVersions) GetString(v string) (*semver.Version, error) {
	key, err := parse(v)
	if err != nil {
		return nil, err
	}
	return vs.Get(*key)
}

func (vs latestVersions) Get(key semver.Version) (*semver.Version, error) {
	v, ok := vs[prepareKey(key)]
	if !ok {
		return nil, ErrNoVersion
	}
	return &v, nil
}

func (vs latestVersions) Latest() *semver.Version {
	var maxValue semver.Version
	for _, value := range vs {
		if value.GreaterThan(&maxValue) {
			maxValue = value
		}
	}
	return &maxValue
}

func (vs latestVersions) Oldest() *semver.Version {
	minValue := *parseFromInt(100000, 0, 0)
	for _, value := range vs {
		if minValue.GreaterThan(&value) {
			minValue = value
		}
	}
	return &minValue
}

func prepareKey(key semver.Version) semver.Version {
	return *parseFromInt(key.Major(), key.Minor(), 0)
}
