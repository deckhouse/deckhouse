/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package taskstarter

import "context"

type Task interface {
	Name() string
	Run(ctx context.Context, errCh chan error)
}
