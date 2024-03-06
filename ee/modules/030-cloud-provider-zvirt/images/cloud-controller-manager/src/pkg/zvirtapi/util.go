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
