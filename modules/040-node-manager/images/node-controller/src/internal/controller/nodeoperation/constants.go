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

	// conditionProgress says where the operation is and why. The phase says what
	// happened; this says who decided so.
	conditionProgress = "Progress"

	// conditionDrainRequested records that this operation has already asked the
	// draining controller for its eviction; the ask cannot be read back off the
	// node once the draining controller clears the request.
	conditionDrainRequested = "DrainRequested"

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

	// retention is how long a finished operation is kept: worth having while
	// someone is still investigating, worthless a day later — without a limit
	// the list only grows (one record per node per change).
	retention = 24 * time.Hour

	// drainingSource prefixes the drains this controller asks for; the node
	// marker adds the operation's identity (see drainMarker) because the two
	// drain annotations are a single slot shared with "bashible" and "user".
	drainingSource = "node-operation"

	// nodeOperationKind is how an eviction names the interruption that asked
	// for it, in an owner reference.
	nodeOperationKind = "NodeOperation"
)
