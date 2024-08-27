/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"context"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/image"
)

type ImageService struct {
	*Service
}

func NewImageService(service *Service) *ImageService {
	return &ImageService{service}
}

func (i *ImageService) GetImageByName(ctx context.Context, name string) (*image.ItemImage, error) {
	var vmImage *image.ItemImage

	err := i.retryer.Do(ctx, func() (bool, error) {
		imageList, err := i.client.CloudAPI().Image().List(ctx, image.ListRequest{
			Name: name,
		})
		if err != nil {
			return false, err
		}

		if len(imageList.Data) == 0 {
			return true, ErrNotFound
		}

		vmImage = &imageList.Data[0]

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return vmImage, nil
}
