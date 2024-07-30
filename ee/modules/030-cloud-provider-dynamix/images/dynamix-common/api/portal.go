/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"
)

type PortalService struct {
	*Service
}

func NewPortalService(service *Service) *PortalService {
	return &PortalService{service}
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
