/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"

	"dynamix-common/retry"
	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/disks"
)

type DiskService struct {
	client  *decort.DecortClient
	retryer retry.Retryer
}

func NewDiskService(client *decort.DecortClient) *DiskService {
	return &DiskService{
		client:  client,
		retryer: retry.NewRetryer(),
	}
}

func (d *DiskService) ListDisksByName(
	ctx context.Context,
	name string,
) ([]disks.ItemDisk, error) {
	var result []disks.ItemDisk

	err := d.retryer.Do(ctx, func() (bool, error) {
		req := disks.ListRequest{
			Name: name,
		}
		resp, err := d.client.CloudAPI().Disks().List(ctx, req)
		if err != nil {
			return false, err
		}
		result = resp.Data
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *DiskService) ListDisksByAccountName(
	ctx context.Context,
	accountName string,
) ([]disks.ItemDisk, error) {
	var result []disks.ItemDisk

	err := d.retryer.Do(ctx, func() (bool, error) {
		req := disks.ListRequest{
			AccountName: accountName,
		}
		resp, err := d.client.CloudAPI().Disks().List(ctx, req)
		if err != nil {
			return false, err
		}
		result = resp.Data
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *DiskService) ListDisksByAccountName(
	ctx context.Context,
	accountName string,
) ([]disks.ItemDisk, error) {
	var result []disks.ItemDisk

	err := d.retryer.Do(ctx, func() (bool, error) {
		req := disks.ListRequest{
			AccountName: accountName,
		}
		resp, err := d.client.CloudAPI().Disks().List(ctx, req)
		if err != nil {
			return false, err
		}
		result = resp.Data
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *DiskService) CreateDisk(
	ctx context.Context,
	accountID uint64,
	gID uint64,
	diskName string,
	size uint64,
	pool string,
	sepID uint64,

) (*disks.ItemDisk, error) {
	var diskID uint64

	err := d.retryer.Do(ctx, func() (bool, error) {
		var err error
		req := disks.CreateRequest{
			AccountID: accountID,
			GID:       gID,
			Name:      diskName,
			Size:      size,
			Pool:      pool,
			SEPID:     sepID,
			Type:      "D",
		}
		diskID, err = d.client.CloudAPI().Disks().Create(ctx, req)
		if err != nil {
			return false, err
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	result, err := d.GetDisk(ctx, diskID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *DiskService) GetDisk(
	ctx context.Context,
	diskID uint64,
) (*disks.ItemDisk, error) {
	var result *disks.ItemDisk

	err := d.retryer.Do(ctx, func() (bool, error) {
		req := disks.ListRequest{
			ByID: diskID,
		}
		resp, err := d.client.CloudAPI().Disks().List(ctx, req)
		if err != nil {
			return false, err
		}

		if resp.EntryCount == 0 {
			return true, ErrNotFound
		}

		result = &resp.Data[0]
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *DiskService) RemoveDisk(
	ctx context.Context,
	diskID uint64,
) error {
	err := d.retryer.Do(ctx, func() (bool, error) {
		req := disks.DeleteRequest{
			DiskID: diskID,
		}

		_, err := d.client.CloudAPI().Disks().Delete(ctx, req)
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

func (d *DiskService) Resize2Disk(
	ctx context.Context,
	diskID uint64,
	size uint64,
) error {
	err := d.retryer.Do(ctx, func() (bool, error) {
		req := disks.ResizeRequest{
			DiskID: diskID,
			Size:   size,
		}
		_, err := d.client.CloudAPI().Disks().Resize2(ctx, req)
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
