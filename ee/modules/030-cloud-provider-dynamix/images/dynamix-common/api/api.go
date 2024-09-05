/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"errors"

	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	sdkconfig "repository.basistech.ru/BASIS/decort-golang-sdk/config"

	"dynamix-common/config"
	"dynamix-common/retry"
)

var ErrNotFound = errors.New("not found")

type DynamixCloudAPI struct {
	Service                *Service
	AccountService         *AccountService
	ComputeService         *ComputeService
	DiskService            *DiskService
	LocationService        *LocationService
	PortalService          *PortalService
	StorageEndpointService *StorageEndpointService
	ExternalNetworkService *ExternalNetworkService
	InternalNetworkService *InternalNetworkService
	LoadBalancerService    *LoadBalancerService
	ResourceGroupService   *ResourceGroupService
}

func NewDynamixCloudAPI(config config.Credentials) (*DynamixCloudAPI, error) {
	decortClient := decort.New(sdkconfig.Config{
		AppID:         config.AppID,
		AppSecret:     config.AppSecret,
		SSOURL:        config.OAuth2URL,
		DecortURL:     config.ControllerURL,
		SSLSkipVerify: config.Insecure,
	})

	service := &Service{
		client:  decortClient,
		retryer: retry.NewRetryer(),
	}

	externalNetworkService := NewExternalNetworkService(service)
	internalNetworkService := NewInternalNetworkService(service)

	return &DynamixCloudAPI{
		Service:                service,
		AccountService:         NewAccountService(service),
		ComputeService:         NewComputeService(service),
		DiskService:            NewDiskService(service),
		LocationService:        NewLocationService(service),
		PortalService:          NewPortalService(service),
		StorageEndpointService: NewStorageEndpointService(service),
		ExternalNetworkService: externalNetworkService,
		InternalNetworkService: internalNetworkService,
		LoadBalancerService: NewLoadBalancerService(
			service,
			externalNetworkService,
			internalNetworkService,
		),
		ResourceGroupService: NewResourceGroupService(service),
	}, nil
}
