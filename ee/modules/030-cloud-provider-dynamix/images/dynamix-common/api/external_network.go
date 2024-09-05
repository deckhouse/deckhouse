/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/extnet"
)

type ExternalNetworkService struct {
	*Service
}

func NewExternalNetworkService(service *Service) *ExternalNetworkService {
	return &ExternalNetworkService{service}
}

func (e *ExternalNetworkService) GetExternalNetworks(ctx context.Context) ([]extnet.ItemExtNet, error) {
	var externalNetworks []extnet.ItemExtNet

	err := e.retryer.Do(ctx, func() (bool, error) {
		externalNetworksList, err := e.client.CloudAPI().ExtNet().List(ctx, extnet.ListRequest{})
		if err != nil {
			return false, err
		}

		externalNetworks = externalNetworksList.Data

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return externalNetworks, nil
}
