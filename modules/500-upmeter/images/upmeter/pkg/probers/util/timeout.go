package util

import (
	"context"
	"sync"
	"time"
)

// DoWithTimer runs jobCb in background and waits until it is done. When timerDuration
// is passed and job is not done yet, onTimerCb is executed.
func DoWithTimer(timerDuration time.Duration, jobCb func(), onTimerCb func()) {
	timer := time.NewTimer(timerDuration)
	defer timer.Stop()

	// Start job in background
	doneCh := make(chan struct{})
	go func() {
		jobCb()
		close(doneCh)
	}()

	// Wait for closed doneCh or for timeout signal.
	for {
		select {
		case <-timer.C:
			onTimerCb()
		case <-doneCh:
			return
		}
	}
}

// SequentialDoWithTimer starts itemHandler for each item in array.
// If itemHandler works more then timerDuration, then itemHandler for next
// item is started. If itemHandler return 1, then all handlers are stopped via context.
func SequentialDoWithTimer(
	parentCtx context.Context,
	timerDuration time.Duration,
	items []string,
	itemHandler func(ctx context.Context, idx int, item string) int,
	timeoutHandler func(idx int, item string),
) {
	wg := sync.WaitGroup{}
	wg.Add(len(items))

	ctx, cancel := context.WithCancel(parentCtx)
	for itemIndex, item := range items {
		go func(idx int, item string) {
			nCtx, _ := context.WithCancel(ctx)
			delayTimer := time.NewTimer(time.Duration(idx) * timerDuration)
			defer func() {
				wg.Done()
				delayTimer.Stop()
			}()
			select {
			case <-nCtx.Done():
				return
			case <-delayTimer.C:
			}
			DoWithTimer(timerDuration, func() {
				result := itemHandler(nCtx, idx, item)
				if result == 1 {
					cancel()
				}
			}, func() {
				timeoutHandler(idx, item)
			})
		}(itemIndex, item)
	}

	wg.Wait()
}
