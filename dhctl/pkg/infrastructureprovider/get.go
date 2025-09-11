// Copyright 2025 Flant JSC
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

package infrastructureprovider

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/tofu"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)


func ExecutorProvider(metaConfig *config.MetaConfig) infrastructure.ExecutorProvider {
	if metaConfig == nil {
		panic("meta config must be provided")
	}

	if metaConfig.ProviderName == "" {
		return func(_ string, logger log.Logger) infrastructure.Executor {
			return infrastructure.NewDummyExecutor(logger)
		}
	}

	if infrastructure.NeedToUseOpentofu(metaConfig) {
		return func(w string, logger log.Logger) infrastructure.Executor {
			return tofu.NewExecutor(w, logger)
		}
	}

	return func(w string, logger log.Logger) infrastructure.Executor {
		return terraform.NewExecutor(w, logger)
	}
}
