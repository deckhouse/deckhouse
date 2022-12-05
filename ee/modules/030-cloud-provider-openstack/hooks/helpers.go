/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/apiversions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// getVolumeTypesArray extract volume types from go hooks
func getVolumeTypesArray() ([]string, error) {
	client, err := clientconfig.NewServiceClient("volume", nil)
	if err != nil {
		return nil, err
	}

	allPages, err := volumetypes.List(client, volumetypes.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	volumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil || len(volumeTypes) == 0 {
		return nil, fmt.Errorf("list of volume types is empty is empty, or an error was returned: %v", err)
	}

	var volumeTypesList []string
	for _, vt := range volumeTypes {
		volumeTypesList = append(volumeTypesList, vt.Name)
	}

	return volumeTypesList, nil
}

var onlineResizeMinVersion = semver.MustParse("3.42")

// isSupportsOnlineDiskResize checks if openstack supports online resize, used as go lib
func isSupportsOnlineDiskResize() (bool, error) {
	client, err := clientconfig.NewServiceClient("volume", nil)
	if err != nil {
		return false, err
	}

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
