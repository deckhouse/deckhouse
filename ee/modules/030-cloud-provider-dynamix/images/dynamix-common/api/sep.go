/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"
	"sort"

	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudbroker/sep"

	"dynamix-common/entity"
	"dynamix-common/retry"
)

type SEPService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewSEPService(client *decort.DecortClient) *SEPService {
	return &SEPService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}
func (c *SEPService) GetSEPByName(ctx context.Context, name string) (*sep.RecordSEP, error) {
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

func extractPoolsFromRecordSEP(item *sep.RecordSEP) []entity.Pool {
	result := []entity.Pool{}
	rawPools, ok := item.Config["pools"]
	if !ok {
		return result
	}

	pools, ok := rawPools.([]struct {
		Name   string
		System bool
	})
	if !ok {
		return result
	}

	for _, pool := range pools {
		if pool.System {
			continue
		}
		result = append(result, entity.Pool{
			Name: pool.Name,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (c *SEPService) ListSEPWithPoolsByGID(ctx context.Context, gID uint64) ([]entity.SEP, error) {
	var result []entity.SEP
	err := c.retryer.Do(ctx, func() (bool, error) {
		req := sep.ListRequest{
			GID: gID,
		}
		items, err := c.client.CloudBroker().SEP().List(ctx, req)
		if err != nil {
			return false, err
		}

		for _, item := range items.Data {
			result = append(result, entity.SEP{
				ID:        item.ID,
				Name:      item.Name,
				IsActive:  item.TechStatus == "ENABLED",
				IsCreated: item.ObjStatus == "CREATED",
				Pools:     extractPoolsFromRecordSEP(&item),
			})
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
