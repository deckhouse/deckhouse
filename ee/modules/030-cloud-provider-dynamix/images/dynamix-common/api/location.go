/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	"dynamix-common/retry"
	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/locations"
)

type LocationService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewLocationService(client *decort.DecortClient) *LocationService {
	return &LocationService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}
func (c *LocationService) GetLocationByName(ctx context.Context, name string) (*locations.ItemLocation, error) {
	var result *locations.ItemLocation

	err := c.retryer.Do(ctx, func() (bool, error) {
		req := locations.ListRequest{
			Name: name,
		}
		items, err := c.client.CloudAPI().Locations().List(ctx, req)
		if err != nil {
			return false, err
		}

		if len(items.Data) == 0 {
			return true, ErrNotFound
		}

		result = &items.Data[0]

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
