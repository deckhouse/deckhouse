package infrastructure

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

func NeedToUseOpentofu(metaConfig *config.MetaConfig) bool {
	return false
}

func IsMasterInstanceDestructiveChanged(_ context.Context, rc plan.ResourceChange, rm map[string]string) bool {
	return false
}

func (r *Runner) getProviderVMTypes() (map[string]string, error) {
	return map[string]string{}, nil
}
