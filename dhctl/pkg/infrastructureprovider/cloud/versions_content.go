package cloud

import (
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/version"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var versionContentProviders = map[string]versionContentProvider{
	vcd.ProviderName: vcd.VersionContentProvider,
}

var contentProviderMutex sync.Mutex

func getVersionContentProvider(s settings.ProviderSettings, provider string) versionContentProvider {
	contentProviderMutex.Lock()
	defer contentProviderMutex.Unlock()

	choicer, ok := versionContentProviders[provider]
	if ok {
		return choicer
	}

	return func(settings settings.ProviderSettings, metaConfig *config.MetaConfig, _ log.Logger) ([]byte, error) {
		if len(settings.Versions()) != 1 {
			return nil, fmt.Errorf("no one version found for provider %s", provider)
		}

		return version.GetVersionContent(s, settings.Versions()[0]), nil
	}
}
