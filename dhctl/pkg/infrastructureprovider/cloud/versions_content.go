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

package cloud

import (
	"context"
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/version"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

var versionContentProviders = map[string]VersionContentProvider{
	vcd.ProviderName: vcd.VersionContentProvider,
}

var contentProviderMutex sync.Mutex

func DefaultVersionContentProvider(ctx context.Context, s settings.ProviderSettings, provider string) VersionContentProvider {
	contentProviderMutex.Lock()
	defer contentProviderMutex.Unlock()

	versionContentProvider, ok := versionContentProviders[provider]
	if ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found custom version choicer for provider %s", provider))
		return versionContentProvider
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("No custom version choicer for provider %s. Using default", provider))

	return func(_ context.Context, settings settings.ProviderSettings, _ *config.MetaConfig) ([]byte, string, error) {
		versions := settings.Versions()
		l := len(versions)
		if l != 1 {
			return nil, "", fmt.Errorf("Expected exactly one version, but found %d for provider %s", l, provider)
		}

		v := versions[0]

		return version.GetVersionContent(s, v), v, nil
	}
}
