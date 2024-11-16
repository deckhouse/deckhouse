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
