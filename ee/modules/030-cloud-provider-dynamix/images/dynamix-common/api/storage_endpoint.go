/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"
	"log"
	"sort"

	"dynamix-common/entity"
	"dynamix-common/retry"
	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudbroker/sep"
)

type StorageEndpointService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewStorageEndpointService(client *decort.DecortClient) *StorageEndpointService {
	return &StorageEndpointService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}

func (c *StorageEndpointService) GetStorageEndpointByName(ctx context.Context, name string) (*sep.RecordSEP, error) {
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

	log.Printf("raw pools: %+v", rawPools)

	pools, ok := rawPools.([]interface{})
	if !ok {
		log.Printf("raw pools cannot type assert")
		return result
	}

	for _, poolI := range pools {
		pool := poolI.(map[string]interface{})
		log.Printf("map pools: %+v", pool)

		if system, ok := pool["system"]; ok && system.(string) == "true" {
			continue
		}

		if name, ok := pool["name"]; ok {
			result = append(result, entity.Pool{
				Name: name.(string),
			})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (c *StorageEndpointService) ListStorageEndpointsWithPoolsByGID(ctx context.Context, gID uint64) ([]entity.StorageEndpoint, error) {
	var result []entity.StorageEndpoint
	err := c.retryer.Do(ctx, func() (bool, error) {
		req := sep.ListRequest{
			GID: gID,
		}
		items, err := c.client.CloudBroker().SEP().List(ctx, req)
		if err != nil {
			return false, err
		}

		for _, item := range items.Data {
			result = append(result, entity.StorageEndpoint{
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
