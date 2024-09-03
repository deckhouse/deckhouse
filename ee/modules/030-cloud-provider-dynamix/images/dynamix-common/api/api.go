/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"errors"

	decort "repository.basistech.ru/BASIS/decort-golang-sdk"
	sdkconfig "repository.basistech.ru/BASIS/decort-golang-sdk/config"

	"dynamix-common/config"
)

var ErrNotFound = errors.New("not found")

type DynamixCloudAPI struct {
	ComputeSvc *ComputeService
}

func NewDynamixCloudAPI(config config.Credentials) (*DynamixCloudAPI, error) {
	return &DynamixCloudAPI{
		ComputeSvc: NewComputeService(decort.New(sdkconfig.Config{
			AppID:         config.AppID,
			AppSecret:     config.AppSecret,
			SSOURL:        config.OAuth2URL,
			DecortURL:     config.ControllerURL,
			SSLSkipVerify: config.Insecure,
		})),
	}, nil
}
