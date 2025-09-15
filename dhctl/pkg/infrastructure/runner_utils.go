package infrastructure

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func NeedToUseOpentofu(metaConfig *config.MetaConfig) bool {
	return false
}
