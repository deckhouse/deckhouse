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

package provider

import (
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

type Provider interface {
	NeedToUseTofu(metaConfig *config.MetaConfig) bool
	IsVMChange(rc plan.ResourceChange) bool
}

var GetInfrastructureSettings = sync.OnceValue[Provider](func() Provider {
	//s, err := loadSettings(infrastructure.GetInfrastructureVersions())
	//if err != nil {
	//	panic(err)
	//}
	//
	//return s

	return nil
})
