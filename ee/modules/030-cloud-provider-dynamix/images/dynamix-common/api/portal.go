/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	"dynamix-common/retry"
	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
)

type PortalService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewPortalService(client *decort.DecortClient) *PortalService {
	return &PortalService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}

func (p *PortalService) Test(ctx context.Context) error {
	err := p.retryer.Do(ctx, func() (bool, error) {
		_, err := p.client.CloudAPI().Locations().GetURL(ctx)
		if err != nil {
			return false, err
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}
