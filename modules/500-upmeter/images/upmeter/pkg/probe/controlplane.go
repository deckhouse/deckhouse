package probe

import (
	"os"
	"time"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
)

func ControlPlane(access kubernetes.Access) []runnerConfig {
	const (
		groupName = "control-plane"
		namespace = "d8-upmeter"
		gcTimeout = 10 * time.Second
	)

	return []runnerConfig{
		{
			group:  groupName,
			probe:  "access",
			check:  "_",
			period: 5 * time.Second,
			config: checker.ControlPlaneAvailable{
				Access:  access,
				Timeout: 5 * time.Second,
			},
		}, {
			group:  groupName,
			probe:  "basic-functionality",
			check:  "_",
			period: 5 * time.Second,
			config: checker.ConfigMapLifecycle{
				Access:                   access,
				Timeout:                  5 * time.Second,
				Namespace:                namespace,
				GarbageCollectionTimeout: gcTimeout,
			},
		}, {
			group:  groupName,
			probe:  "namespace",
			check:  "_",
			period: time.Minute,
			config: checker.NamespaceLifecycle{
				Access:                   access,
				CreationTimeout:          5 * time.Second,
				DeletionTimeout:          time.Minute,
				GarbageCollectionTimeout: gcTimeout,
			},
		}, {
			group:  groupName,
			probe:  "controller-manager",
			check:  "_",
			period: time.Minute,
			config: checker.DeploymentLifecycle{
				Access:                    access,
				Namespace:                 namespace,
				DeploymentCreationTimeout: 5 * time.Second,
				DeploymentDeletionTimeout: 5 * time.Second,
				PodAppearTimeout:          10 * time.Second,
				PodDisappearTimeout:       10 * time.Second,
				GarbageCollectionTimeout:  gcTimeout,
			},
		}, {
			group:  groupName,
			probe:  "scheduler",
			check:  "_",
			period: time.Minute,
			config: checker.PodLifecycle{
				Access:                   access,
				Namespace:                namespace,
				Node:                     os.Getenv("NODE_NAME"),
				CreationTimeout:          5 * time.Second,
				SchedulingTimeout:        20 * time.Second,
				DeletionTimeout:          5 * time.Second,
				GarbageCollectionTimeout: gcTimeout,
			},
		},
	}
}
