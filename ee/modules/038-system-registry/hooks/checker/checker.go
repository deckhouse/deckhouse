/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checker

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func checkRegistry(ctx context.Context, queue *registryQueue, params RegistryParams) error {
	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		checkedCount int64
	)

	if params.Address == "" {
		return fmt.Errorf("params addr is %q", params.Address)
	}

	worker := func(ctx context.Context, done func()) {
		defer done()

		for {
			mu.Lock()
			if len(queue.Items) == 0 {
				mu.Unlock()
				break
			}

			if ctx.Err() != nil && checkedCount > 0 {
				mu.Unlock()
				break
			}

			item := queue.Items[0]
			queue.Items = queue.Items[1:]
			mu.Unlock()

			time.Sleep(1 * time.Second)
			ri := rand.Intn(10)
			isErr := ri%8 == 0 || ri%9 == 0

			if isErr {
				item.Error = fmt.Sprintf("Check error: %v", ri)
			}

			mu.Lock()
			if isErr {
				queue.Retry = append(queue.Retry, item)
			} else {
				queue.Processed++
			}
			checkedCount++
			mu.Unlock()
		}
	}

	for range parallelizmPerRegistry {
		wg.Add(1)
		go worker(ctx, wg.Done)
	}
	wg.Wait()

	return nil
}
