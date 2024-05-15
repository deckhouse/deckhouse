/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	// "k8s.io/client-go/tools/record"
	"os"
	"system-registry-manager/internal/config"
	"time"
)

func StartLeaderElection(
	ctx context.Context,
	// recorder record.EventRecorder,
	callbacks leaderelection.LeaderCallbacks,
) error {

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %v", err)
	}

	cfg := config.GetConfig()

	lockName := "system-registry-manager"
	identity := "system-registry-manager-" + hostname
	namespace := cfg.LeaderElection.Namespace

	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		namespace,
		lockName,
		cfg.K8sClient.CoreV1(),
		cfg.K8sClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: identity,
			// EventRecorder: recorder,
		},
	)
	if err != nil {
		return fmt.Errorf("error creating leases resource lock: %v", err)
	}

	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            rl,
		LeaseDuration:   time.Duration(cfg.LeaderElection.LeaseDurationSeconds * int(time.Second)),
		RenewDeadline:   time.Duration(cfg.LeaderElection.RenewDeadlineSeconds * int(time.Second)),
		RetryPeriod:     time.Duration(cfg.LeaderElection.RetryPeriodSeconds * int(time.Second)),
		ReleaseOnCancel: true,
		Callbacks:       callbacks,
	})
	if err != nil {
		return fmt.Errorf("error creating leader elector: %v", err)
	}

	le.Run(ctx)
	return nil
}
