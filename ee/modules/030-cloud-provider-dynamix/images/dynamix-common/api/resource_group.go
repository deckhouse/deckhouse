/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/rg"
)

type ResourceGroupService struct {
	*Service
}

func NewResourceGroupService(service *Service) *ResourceGroupService {
	return &ResourceGroupService{service}
}

func (r *ResourceGroupService) GetResourceGroup(ctx context.Context, resourceGroupName string) (*rg.ItemResourceGroup, error) {
	var resourceGroup *rg.ItemResourceGroup

	err := r.retryer.Do(ctx, func() (bool, error) {
		resourceGroups, err := r.client.CloudAPI().RG().List(ctx, rg.ListRequest{
			Name: resourceGroupName,
		})
		if err != nil {
			return false, err
		}

		if len(resourceGroups.Data) == 0 {
			return true, ErrNotFound
		}

		resourceGroup = &resourceGroups.Data[0]

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return resourceGroup, nil
}
