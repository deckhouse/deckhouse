package openstack

import (
	"encoding/json"
	"fmt"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"os"
)

func GetVolumeTypes() error {
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

	allPages, err := volumetypes.List(client, volumetypes.ListOpts{}).AllPages()
	if err != nil{
		return err
	}

	volumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil || len(volumeTypes) == 0 {
		return fmt.Errorf("list of volume types is empty is empty, or an error was returned: %v", err)
	}

	var volumeTypesList []string
	for _, vt := range volumeTypes{
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
