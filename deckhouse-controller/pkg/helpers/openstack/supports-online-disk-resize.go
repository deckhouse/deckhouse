// Copyright 2021 Flant CJSC
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
	"os"

	"github.com/blang/semver"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/apiversions"
)

var onlineResizeMinVersion = semVerMustParseTolerant("3.42")

func semVerMustParseTolerant(ver string) semver.Version {
	semVersion, err := semver.ParseTolerant(ver)
	if err != nil {
		panic(err)
	}

	return semVersion
}

func SupportsOnlineDiskResize() error {
	opts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return err
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return err
	}

	client, err := openstack.NewBlockStorageV3(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return err
	}

	allPages, err := apiversions.List(client).AllPages()
	if err != nil {
		return fmt.Errorf("unable to get API versions: %s", err)
	}

	allVersions, err := apiversions.ExtractAPIVersions(allPages)
	if err != nil {
		return fmt.Errorf("unable to extract API versions: %s", err)
	}

	var currentVersion string
	for _, version := range allVersions {
		if version.ID == "v3.0" {
			currentVersion = version.Version
			break
		}
	}

	if currentVersion == "" {
		return fmt.Errorf("cannot determine current API version for 3.0 block-storage")
	}

	currentVersionSemVer := semVerMustParseTolerant(currentVersion)

	var stdout string
	if currentVersionSemVer.GE(onlineResizeMinVersion) {
		stdout = "yes"
	} else {
		stdout = "no"
	}

	_, err = fmt.Print(stdout)
	if err != nil {
		return err
	}

	return nil
}
