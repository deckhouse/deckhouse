// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
)

func GetMetaConfig(ctx context.Context, kubeCl *client.KubernetesClient, logger log.Logger) (*config.MetaConfig, error) {
	metaConfig, err := config.ParseConfigFromCluster(
		ctx,
		kubeCl,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(logger),
		),
	)
	if err != nil {
		return nil, err
	}

	metaConfig.UUID, err = infrastructurestate.GetClusterUUID(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	return metaConfig, nil
}
