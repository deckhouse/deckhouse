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

package nodeoperation

import "time"

const (
	controllerName = "node-operation"

	// conditionDrained is this controller's account of the eviction. It also
	// records that the eviction was asked for: the ask cannot be read back off
	// the node once the draining controller clears the request.
	conditionDrained = "Drained"

	// conditionNodeReported is the only thing the node writes: how the operation
	// it was handed ended. The phase is derived from it here.
	conditionNodeReported = "NodeReported"

	// reasonRebooting keeps an operation InProgress after the node reported
	// RebootStarted, until the node is back.
	reasonRebooting = "Rebooting"

	// reasonReleased completes a Disruption the node gave back without applying
	// its revision. The node's NodeConfig says why.
	reasonReleased = "Released"

	// conditionRetry says, on a failed Disruption, whether node-manager makes
	// another attempt and when. nodelet reads it off the newest attempt to say
	// the same on the NodeConfig.
	conditionRetry               = "Retry"
	reasonRetryCreated           = "Created"
	reasonRetrySuperseded        = "Superseded"
	reasonRetryNotRequested      = "NotRequested"
	reasonRetryAttemptsExhausted = "AttemptsExhausted"
	reasonRetryScheduled         = "Scheduled"

	// operationTimeout bounds each stretch of an operation: the wait for the
	// node to act, and — on top of the group's drain timeout — the wait for
	// the eviction. A node silent this long is not coming back on its own.
	operationTimeout = 30 * time.Minute

	// waitPollInterval is how often an operation waiting on an eviction looks
	// again: events are the normal wake-up, this only covers a dropped event
	// and lets the deadline above fire on time.
	waitPollInterval = 30 * time.Second

	// minRequeue is the floor under a requeue computed from a deadline. To
	// controller-runtime a RequeueAfter of zero or less means "do not requeue",
	// so a deadline that passed mid-pass would strand the operation.
	minRequeue = time.Second

	// drainingSource prefixes the drains this controller asks for; the node
	// marker adds the operation's identity (see drainMarker) because the two
	// drain annotations are a single slot shared with "bashible" and "user".
	drainingSource = "node-operation"

	// nodeOperationKind is how a Drain the previous controller created names
	// the operation it evicts for, in an owner reference.
	nodeOperationKind = "NodeOperation"
)
