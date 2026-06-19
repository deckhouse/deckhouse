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
			virtProbe.VirtualMachineClassName,
			false,
			virtualMachineLifecycleTimeouts{
				period:                      5 * time.Minute,
				waitVirtualImage:            25 * time.Second,
				waitVirtualDisk:             60 * time.Second,
				waitVirtualMachine:          45 * time.Second,
				waitVirtualMachineMigration: time.Minute,
				waitDeletion:                30 * time.Second,
				waitNamespaceDeleted:        20 * time.Second,
				total:                       3*time.Minute + 30*time.Second,
			},
		),
		virtualMachineLifecycleRunner(
			access,
			controlPlanePinger,
			logger,
			checker.VirtualizationLifecycleProbeName,
			"upmeter-vm-lifecycle",
			virtProbe.VirtualImageURL,
			virtProbe.VirtualMachineClassName,
			true,
			virtualMachineLifecycleTimeouts{
				period:                      15 * time.Minute,
				waitVirtualImage:            25 * time.Second,
				waitVirtualDisk:             50 * time.Second,
				waitVirtualMachine:          30 * time.Second,
				waitVirtualMachineMigration: 40 * time.Second,
				waitDeletion:                30 * time.Second,
				waitNamespaceDeleted:        30 * time.Second,
				total:                       8 * time.Minute,
			},
		),
	}
}

type virtualMachineLifecycleTimeouts struct {
	period                      time.Duration
	waitVirtualImage            time.Duration
	waitVirtualDisk             time.Duration
	waitVirtualMachine          time.Duration
	waitVirtualMachineMigration time.Duration
	waitDeletion                time.Duration
	waitNamespaceDeleted        time.Duration
	total                       time.Duration
}

func virtualMachineLifecycleRunner(
	access kubernetes.Access,
	preflight check.Checker,
	logger *logrus.Logger,
	probeName,
	namespaceSuffix,
	virtualImageURL,
	virtualMachineClassName string,
	verifyLifecycle bool,
	timeouts virtualMachineLifecycleTimeouts,
) runnerConfig {
	return runnerConfig{
		group:  checker.VirtualizationGroupName,
		probe:  probeName,
		check:  "virtual-machine-lifecycle",
		period: timeouts.period,
		config: checker.VirtualMachineLifecycle{
			Access:           access,
			PreflightChecker: preflight,
			Logger: logrus.NewEntry(logger).WithFields(logrus.Fields{
				"group": checker.VirtualizationGroupName,
				"probe": probeName,
				"check": "virtual-machine-lifecycle",
			}),
			AgentID:                 run.ID(),
			Namespace:               run.StaticIdentifier(namespaceSuffix),
			ProbeName:               probeName,
			VirtualImageName:        checker.VirtualizationImageName,
			VirtualImageURL:         virtualImageURL,
			VirtualMachineClassName: virtualMachineClassName,
			VerifyLifecycle:         verifyLifecycle,

			RequestTimeout:                     5 * time.Second,
			WaitVirtualImageTimeout:            timeouts.waitVirtualImage,
			WaitVirtualDiskTimeout:             timeouts.waitVirtualDisk,
			WaitVirtualMachineTimeout:          timeouts.waitVirtualMachine,
			WaitVirtualMachineMigrationTimeout: timeouts.waitVirtualMachineMigration,
			WaitDeletionTimeout:                timeouts.waitDeletion,
			WaitNamespaceDeletedTimeout:        timeouts.waitNamespaceDeleted,
			Timeout:                            timeouts.total,
		},
	}
}
