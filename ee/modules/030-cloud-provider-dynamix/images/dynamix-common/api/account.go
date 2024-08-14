/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/account"

	"dynamix-common/retry"
)

type AccountService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewAccountService(client *decort.DecortClient) *AccountService {
	return &AccountService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}
func (c *AccountService) GetAccountByName(ctx context.Context, name string) (*account.ItemAccount, error) {
	var result *account.ItemAccount

	err := c.retryer.Do(ctx, func() (bool, error) {
		req := account.ListRequest{
			Name: name,
		}
		items, err := c.client.CloudAPI().Account().List(ctx, req)
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
