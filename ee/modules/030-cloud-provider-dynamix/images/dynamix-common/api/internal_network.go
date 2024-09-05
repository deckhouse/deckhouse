/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/vins"
)

type InternalNetworkService struct {
	*Service
}

func NewInternalNetworkService(service *Service) *InternalNetworkService {
	return &InternalNetworkService{service}
}

func (i *InternalNetworkService) GetInternalNetworks(ctx context.Context) ([]vins.ItemVINS, error) {
	var internalNetworks []vins.ItemVINS

	err := i.retryer.Do(ctx, func() (bool, error) {
		internalNetworksList, err := i.client.CloudAPI().VINS().List(ctx, vins.ListRequest{
			RGID: i.resourceGroupID,
		})
		if err != nil {
			return false, err
		}

		internalNetworks = internalNetworksList.Data

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return internalNetworks, nil
}
