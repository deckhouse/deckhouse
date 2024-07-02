/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"os"
	pkg_cfg "system-registry-manager/pkg/cfg"
	"time"
)

const componentName = "system-registry-manager"

func StartLeaderElection(
	ctx context.Context,
	recorder record.EventRecorder,
	callbacks leaderelection.LeaderCallbacks,
	identity string,
) error {

	cfg := pkg_cfg.GetConfig()

	lockName := componentName
	namespace := cfg.Manager.Namespace

	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		namespace,
		lockName,
		cfg.K8sClient.CoreV1(),
		cfg.K8sClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      identity,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		return fmt.Errorf("error creating leases resource lock: %v", err)
	}

	logrus.Info("StartLeaderElection ::NewLeaderElector\n")
	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            rl,
		LeaseDuration:   time.Duration(cfg.Manager.LeaderElection.LeaseDurationSeconds * int(time.Second)),
		RenewDeadline:   time.Duration(cfg.Manager.LeaderElection.RenewDeadlineSeconds * int(time.Second)),
		RetryPeriod:     time.Duration(cfg.Manager.LeaderElection.RetryPeriodSeconds * int(time.Second)),
		ReleaseOnCancel: true,
		Callbacks:       callbacks,
	})
	logrus.Info("StartLeaderElection :: After NewLeaderElector\n")
	if err != nil {
		return fmt.Errorf("error creating leader elector: %v", err)
	}

	le.Run(ctx)
	return nil
}

func NewLeaderElectionRecorder(logEntry *logrus.Entry) record.EventRecorder {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(logEntry.Infof)
	return broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: componentName})
}

func NewIdentityForLeaderElection() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %v", err)
	}
	return hostname, nil
}
