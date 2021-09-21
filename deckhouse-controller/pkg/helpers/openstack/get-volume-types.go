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
	"encoding/json"
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

func GetVolumeTypes() error {
	client, err := clientconfig.NewServiceClient("volume", nil)

	if err != nil {
		return err
	}

	allPages, err := volumetypes.List(client, volumetypes.ListOpts{}).AllPages()
	if err != nil {
		return err
	}

	volumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil || len(volumeTypes) == 0 {
		return fmt.Errorf("list of volume types is empty is empty, or an error was returned: %v", err)
	}

	var volumeTypesList []string
	for _, vt := range volumeTypes {
		volumeTypesList = append(volumeTypesList, vt.Name)
	}

	jsonList, err := json.Marshal(volumeTypesList)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(jsonList)
	if err != nil {
		return err
	}

	return nil
}
