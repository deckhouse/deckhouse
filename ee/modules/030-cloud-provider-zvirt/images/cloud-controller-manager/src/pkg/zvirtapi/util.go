/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package zvirtapi

import (
	"context"

	ovirtclient "github.com/ovirt/go-ovirt-client/v3"
)

func getRetryStrategy(ctx context.Context) []ovirtclient.RetryStrategy {
	return []ovirtclient.RetryStrategy{
		ovirtclient.AutoRetry(),
		ovirtclient.ContextStrategy(ctx),
	}
}
