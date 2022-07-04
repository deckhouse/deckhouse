/*
Copyright 2021 Flant JSC

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
	"time"

	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
	"d8.io/upmeter/pkg/probe/run"
)

func initControlPlane(access kubernetes.Access) []runnerConfig {
	const (
		groupControlPlane = "control-plane"
		namespace         = "d8-upmeter"
		gcTimeout         = 10 * time.Second
		cpTimeout         = 5 * time.Second
	)

	return []runnerConfig{
		{
			group:  groupControlPlane,
			probe:  "apiserver",
			check:  "_",
			period: 5 * time.Second,
			config: checker.ControlPlaneAvailable{
				Access:  access,
				Timeout: cpTimeout,
			},
		}, {
			group:  groupControlPlane,
			probe:  "basic-functionality",
			check:  "_",
			period: 5 * time.Second,
			config: checker.ConfigMapLifecycle{
				Access:                    access,
				Timeout:                   5 * time.Second,
				Namespace:                 namespace,
				GarbageCollectionTimeout:  gcTimeout,
				ControlPlaneAccessTimeout: cpTimeout,
			},
		}, {
			group:  groupControlPlane,
			probe:  "namespace",
			check:  "_",
			period: time.Minute,
			config: checker.NamespaceLifecycle{
				Access:                    access,
				CreationTimeout:           5 * time.Second,
				DeletionTimeout:           time.Minute,
				GarbageCollectionTimeout:  gcTimeout,
				ControlPlaneAccessTimeout: cpTimeout,
			},
		}, {
			group:  groupControlPlane,
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
				ControlPlaneAccessTimeout: cpTimeout,
			},
		}, {
			group:  groupControlPlane,
			probe:  "scheduler",
			check:  "_",
			period: time.Minute,
			config: checker.PodLifecycle{
				Access:                    access,
				Namespace:                 namespace,
				Node:                      access.SchedulerProbeNode(),
				CreationTimeout:           5 * time.Second,
				SchedulingTimeout:         20 * time.Second,
				DeletionTimeout:           20 * time.Second,
				GarbageCollectionTimeout:  gcTimeout,
				ControlPlaneAccessTimeout: cpTimeout,
			},
		}, {
			group:  groupControlPlane,
			probe:  "cert-manager",
			check:  "_",
			period: time.Minute,
			config: checker.CertificateSecretLifecycle{
				Access:                    access,
				Namespace:                 namespace,
				AgentID:                   run.ID(),
				CreationTimeout:           5 * time.Second,
				DeletionTimeout:           20 * time.Second,
				ControlPlaneAccessTimeout: cpTimeout,
			},
		},
	}
}
