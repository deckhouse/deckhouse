/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import "context"

type leaderRunnableFunc func(context.Context) error

// Start implements Runnable.
func (lr leaderRunnableFunc) Start(ctx context.Context) error {
	return lr(ctx)
}

// NeedLeaderElection implements LeaderElectionRunnable
func (lr leaderRunnableFunc) NeedLeaderElection() bool {
	return true
}
