// Copyright 2021 Flant JSC
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

package openstack

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/apiversions"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

var onlineResizeMinVersion = semver.MustParse("3.42")

// IsSupportsOnlineDiskResize checks if openstack supports online resize, used as go lib
func IsSupportsOnlineDiskResize() (bool, error) {
	client, err := clientconfig.NewServiceClient("volume", nil)

	allPages, err := apiversions.List(client).AllPages()
	if err != nil {
		return false, fmt.Errorf("unable to get API versions: %s", err)
	}

	allVersions, err := apiversions.ExtractAPIVersions(allPages)
	if err != nil {
		return false, fmt.Errorf("unable to extract API versions: %s", err)
	}

	var currentVersion string
	for _, version := range allVersions {
		if version.ID == "v3.0" {
			currentVersion = version.Version
			break
		}
	}

	if currentVersion == "" {
		return false, fmt.Errorf("cannot determine current API version for 3.0 block-storage")
	}

	currentVersionSemVer := semver.MustParse(currentVersion)

	if currentVersionSemVer.GreaterThan(onlineResizeMinVersion) || currentVersionSemVer.Equal(onlineResizeMinVersion) {
		return true, nil
	}

	return false, nil
}

// SupportsOnlineDiskResize cli version of IsSupportsOnlineDiskResize
func SupportsOnlineDiskResize() error {
	isSupported, err := IsSupportsOnlineDiskResize()
	if err != nil {
		return err
	}

	stdout := "yes"
	if !isSupported {
		stdout = "no"
	}

	_, err = fmt.Print(stdout)
	if err != nil {
		return err
	}

	return nil
}
