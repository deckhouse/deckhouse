/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudbroker/sep"

	"dynamix-common/retry"
)

type SepService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewSepService(client *decort.DecortClient) *SepService {
	return &SepService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}
func (c *SepService) GetLocationByName(ctx context.Context, name string) (*sep.RecordSEP, error) {
	var result *sep.RecordSEP

	err := c.retryer.Do(ctx, func() (bool, error) {
		req := sep.ListRequest{
			Name: name,
		}
		items, err := c.client.CloudBroker().SEP().List(ctx, req)
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
