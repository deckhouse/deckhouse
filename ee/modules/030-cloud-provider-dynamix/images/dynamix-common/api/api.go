/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"errors"

	"dynamix-common/config"
	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	sdkconfig "repository.basistech.ru/BASIS/decort-golang-sdk/config"
)

var ErrNotFound = errors.New("not found")

type DynamixCloudAPI struct {
	AccountService         *AccountService
	ComputeSvc             *ComputeService
	DiskService            *DiskService
	LocationService        *LocationService
	PortalSvc              *PortalService
	StorageEndpointService *StorageEndpointService
}

func NewDynamixCloudAPI(config config.Credentials) (*DynamixCloudAPI, error) {
	decortClient := decort.New(sdkconfig.Config{
		AppID:         config.AppID,
		AppSecret:     config.AppSecret,
		SSOURL:        config.OAuth2URL,
		DecortURL:     config.ControllerURL,
		SSLSkipVerify: config.Insecure,
	})
	return &DynamixCloudAPI{
		AccountService:         NewAccountService(decortClient),
		ComputeSvc:             NewComputeService(decortClient),
		DiskService:            NewDiskService(decortClient),
		LocationService:        NewLocationService(decortClient),
		PortalSvc:              NewPortalService(decortClient),
		StorageEndpointService: NewStorageEndpointService(decortClient),
	}, nil
}
