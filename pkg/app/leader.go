// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import "time"

// Leader election settings for the deckhouse-controller HA lease.
// The durations are kept in seconds to match the leaderelection API call sites.
const (
	// LeaseName is the name of the leader-election Lease.
	LeaseName = "deckhouse-leader-election"
	// LeaseDurationSeconds is the duration that non-leaders wait before acquiring leadership.
	LeaseDurationSeconds = 35
	// RenewDeadlineSeconds is the deadline for the leader to renew leadership.
	RenewDeadlineSeconds = 30
	// RetryPeriodSeconds is the interval between leadership acquisition attempts.
	RetryPeriodSeconds = 10

	// BootstrapLockName is the ConfigMap used as the bootstrap lock.
	BootstrapLockName = "deckhouse-bootstrap-lock"
)

// GracefulShutdownTimeout bounds the controller-runtime manager shutdown.
const GracefulShutdownTimeout = 10 * time.Second
