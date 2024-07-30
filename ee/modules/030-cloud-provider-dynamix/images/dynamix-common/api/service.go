/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	decort "repository.basistech.ru/BASIS/decort-golang-sdk"

	"dynamix-common/retry"
)

type Service struct {
	client          *decort.DecortClient
	retryer         retry.Retryer
	resourceGroupID uint64
}

func (s *Service) SetResourceGroupID(id uint64) {
	s.resourceGroupID = id
}
