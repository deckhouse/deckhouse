/*
Copyright 2026 Flant JSC

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

	"github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/checker"
	"d8.io/upmeter/pkg/probe/run"
)

func initVirtualization(access kubernetes.Access, preflight checker.Doer, virtProbe VirtualizationProbeConfig, logger *logrus.Logger) []runnerConfig {
	const controlPlaneTimeout = 5 * time.Second

	controlPlanePinger := checker.DoOrUnknown(controlPlaneTimeout, preflight)

	return []runnerConfig{
		virtualMachineLifecycleRunner(
			access,
			controlPlanePinger,
			logger,
			checker.VirtualizationCreationProbeName,
			"upmeter-vm-creation",
			virtProbe.VirtualImageURL,
			false,
		),
		virtualMachineLifecycleRunner(
			access,
			controlPlanePinger,
			logger,
			checker.VirtualizationMigrationProbeName,
			"upmeter-vm-migration",
			virtProbe.VirtualImageURL,
			true,
		),
	}
}

func virtualMachineLifecycleRunner(
	access kubernetes.Access,
	preflight check.Checker,
	logger *logrus.Logger,
	probeName,
	namespaceSuffix,
	virtualImageURL string,
	verifyMigration bool,
) runnerConfig {
	return runnerConfig{
		group:  checker.VirtualizationGroupName,
		probe:  probeName,
		check:  "virtual-machine-lifecycle",
		period: 5 * time.Minute,
		config: checker.VirtualMachineLifecycle{
			Access:           access,
			PreflightChecker: preflight,
			Logger: logrus.NewEntry(logger).WithFields(logrus.Fields{
				"group": checker.VirtualizationGroupName,
				"probe": probeName,
				"check": "virtual-machine-lifecycle",
			}),
			AgentID:                    run.ID(),
			Namespace:                  run.StaticIdentifier(namespaceSuffix),
			ProbeName:                  probeName,
			VirtualImageName:           checker.VirtualizationImageName,
			VirtualImageURL:            virtualImageURL,
			VerifyMigration:            verifyMigration,

			RequestTimeout:                     5 * time.Second,
			WaitVirtualImageTimeout:            30 * time.Second,
			WaitVirtualDiskTimeout:             60 * time.Second,
			WaitVirtualMachineTimeout:          30 * time.Second,
			WaitVirtualMachineMigrationTimeout: time.Minute,
			WaitDeletionTimeout:                30 * time.Second,
			WaitNamespaceDeletedTimeout:        30 * time.Second,
			Timeout:                            6 * time.Minute,
		},
	}
}
