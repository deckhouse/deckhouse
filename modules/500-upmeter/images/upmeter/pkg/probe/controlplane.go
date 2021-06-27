/*
Copyright 2021 Flant CJSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
