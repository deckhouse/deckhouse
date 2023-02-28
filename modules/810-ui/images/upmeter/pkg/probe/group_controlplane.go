/*
Copyright 2023 Flant JSC

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

func initControlPlane(access kubernetes.Access, preflight checker.Doer) []runnerConfig {
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
				VersionGetter: preflight,
				Timeout:       cpTimeout,
			},
		}, {
			group:  groupControlPlane,
			probe:  "basic-functionality",
			check:  "_",
			period: 5 * time.Second,
			config: checker.ConfigMapLifecycle{
				Access:    access,
				Preflight: preflight,
				Namespace: namespace,

				AgentID: run.ID(),
				Name:    run.StaticIdentifier("upmeter-probe-basic"),

				Timeout: 5 * time.Second,
			},
		}, {
			group:  groupControlPlane,
			probe:  "namespace",
			check:  "_",
			period: time.Minute,
			config: checker.NamespaceLifecycle{
				Access:    access,
				Preflight: preflight,

				AgentID: run.ID(),
				Name:    run.StaticIdentifier("upmeter-probe-namespace"),

				CreationTimeout: 5 * time.Second,
				DeletionTimeout: time.Minute,
			},
		}, {
			group:  groupControlPlane,
			probe:  "controller-manager",
			check:  "_",
			period: time.Minute,
			config: checker.StatefulSetPodLifecycle{
				Access:    access,
				Preflight: preflight,
				Namespace: namespace,

				AgentID: run.ID(),
				Name:    run.StaticIdentifier("upmeter-probe-controller-manager"),

				CreationTimeout:      5 * time.Second,
				DeletionTimeout:      5 * time.Second,
				PodTransitionTimeout: 10 * time.Second,
			},
		}, {
			group:  groupControlPlane,
			probe:  "scheduler",
			check:  "_",
			period: time.Minute,
			config: checker.PodScheduling{
				Access:    access,
				Preflight: preflight,
				Namespace: namespace,

				Node:  access.SchedulerProbeNode(),
				Image: access.SchedulerProbeImage(),

				AgentID: run.ID(),
				Name:    run.StaticIdentifier("upmeter-probe-scheduler"),

				CreationTimeout: 5 * time.Second,
				DeletionTimeout: 5 * time.Second,
				ScheduleTimeout: 20 * time.Second,
			},
		}, {
			group:  groupControlPlane,
			probe:  "cert-manager",
			check:  "_",
			period: time.Minute,
			config: checker.CertificateSecretLifecycle{
				Access:    access,
				Preflight: preflight,
				Namespace: namespace,

				AgentID: run.ID(),
				Name:    run.StaticIdentifier("upmeter-probe-cert-manager"),

				CreationTimeout:         5 * time.Second,
				DeletionTimeout:         5 * time.Second,
				SecretTransitionTimeout: time.Minute,
			},
		},
	}
}
