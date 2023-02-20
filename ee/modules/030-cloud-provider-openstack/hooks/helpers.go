/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"errors"
	"fmt"
	"os"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
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

func initOpenstackEnvs(input *go_hook.HookInput) error {
	osAuthURL, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.authURL")
	if !ok {
		return errors.New("cloudProviderOpenstack.internal.connection.authURL required")
	}
	err := os.Setenv("OS_AUTH_URL", osAuthURL.String())
	if err != nil {
		return err
	}

	osUsername, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.username")
	if !ok {
		return errors.New("cloudProviderOpenstack.internal.connection.username required")
	}
	err = os.Setenv("OS_USERNAME", osUsername.String())
	if err != nil {
		return err
	}

	osPassword, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.password")
	if !ok {
		return errors.New("cloudProviderOpenstack.internal.connection.password required")
	}
	err = os.Setenv("OS_PASSWORD", osPassword.String())
	if err != nil {
		return err
	}

	osDomainName, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.domainName")
	if !ok {
		return errors.New("cloudProviderOpenstack.internal.connection.domainName required")
	}
	err = os.Setenv("OS_DOMAIN_NAME", osDomainName.String())
	if err != nil {
		return err
	}

	osProjectName, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.tenantName")
	if ok && osProjectName.String() != "" {
		err = os.Setenv("OS_PROJECT_NAME", osProjectName.String())
		if err != nil {
			return err
		}
	}

	osProjectID, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.tenantID")
	if ok && osProjectID.String() != "" {
		err = os.Setenv("OS_PROJECT_ID", osProjectID.String())
		if err != nil {
			return err
		}
	}

	osRegionName, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.region")
	if !ok {
		return errors.New("cloudProviderOpenstack.internal.connection.region required")
	}
	err = os.Setenv("OS_REGION_NAME", osRegionName.String())
	if err != nil {
		return err
	}

	caCert, ok := input.Values.GetOk("cloudProviderOpenstack.internal.connection.caCert")
	if ok && caCert.String() != "" {
		err = os.WriteFile("/tmp/openstack_ca.crt", []byte(caCert.String()), 0644)
		if err != nil {
			return err
		}
		err = os.Setenv("OS_CACERT", "/tmp/openstack_ca.crt")
		if err != nil {
			return err
		}
	}

	return nil
}
