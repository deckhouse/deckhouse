package commands

import (
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func cleanupSSHProvider(
	ctx context.Context,
	sshProviderInitializer *providerinitializer.SSHProviderInitializer,
) {
	if sshProviderInitializer == nil {
		return
	}

	if err := sshProviderInitializer.Cleanup(ctx); err != nil {
		log.WarnF("failed to cleanup ssh provider: %v", err)
	}
}
