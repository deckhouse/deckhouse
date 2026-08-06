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

import (
	"math"
	"time"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

const (
	controllerName = "node-operation"

	// conditionProgress says where the operation is and why. The phase says what
	// happened; this says who decided so.
	conditionProgress = "Progress"

	// conditionDrainRequested records that this operation has already asked the
	// draining controller for its eviction. The ask cannot be read back off the
	// node: the draining controller clears the request when it reports the
	// workload gone, and the marker it leaves behind outlives the operation that
	// asked for it.
	conditionDrainRequested = "DrainRequested"

	// operationTimeout bounds each stretch of an operation: the wait for the node
	// to carry it out, and — on top of the group's own drain timeout — the wait
	// for its eviction to finish. A node that says nothing for this long is not
	// coming back on its own, and an operation left open keeps it out of the
	// scheduler.
	operationTimeout = 30 * time.Minute

	// defaultDrainTimeout is what the draining controller falls back to when the
	// group sets no nodeDrainTimeoutSecond. Kept as its own copy: the two
	// controllers agree on a contract, and reaching into the other's unexported
	// constant is not one. A test pins the two values together.
	defaultDrainTimeout = 10 * time.Minute

	// maxDrainTimeout is the largest bound that can be expressed at all: past it
	// the multiplication of seconds into a Duration overflows and comes out
	// negative, which would put the deadline before the drain began. That is
	// around 292 years, so it is a guard against nonsense rather than a limit
	// anyone will meet.
	//
	// It is deliberately not a policy cap. The policy cap belongs in the CRD,
	// which holds nodeDrainTimeoutSecond to two hours; a lower cap here would be
	// worse than none, because the draining controller runs to whatever the group
	// says and an operation that gave up sooner would walk away from a drain that
	// is still going.
	maxDrainTimeout = time.Duration(math.MaxInt64)

	// waitPollInterval is how often an operation waiting on an eviction looks
	// again. The events it waits on are the normal wake-up; this only keeps one
	// dropped event from stranding the operation until the manager's next full
	// resync, and lets the deadline above fire on time.
	waitPollInterval = 30 * time.Second

	// minRequeue is the floor under a requeue computed from a deadline. To
	// controller-runtime a RequeueAfter of zero or less means "do not requeue",
	// so a deadline that passed mid-pass would strand the operation.
	minRequeue = time.Second

	// retention is how long a finished operation is kept. It is the record of
	// what was done to a node, which is worth having while someone is still
	// looking into what happened, and worth nothing a day later — a cluster that
	// reboots nodes or rolls configs out produces one of these per node per
	// change, so without a limit the list only grows.
	retention = 24 * time.Hour

	// operationNodeLabel names the node an operation is for; shared with the
	// creator (nodeconfig) so the lookup contract cannot drift.
	operationNodeLabel = v1alpha1.NodeOperationNodeLabel

	// drainingSource prefixes the drains this controller asks for. The marker
	// written on the node adds the operation's own identity to it — see
	// drainMarker — because the two drain annotations are a single slot on a
	// shared object: without identity, every operation on a node reads, and
	// clears, every other one's. The mutable path writes "bashible" and "user"
	// into the same slot, which no marker of ours can equal.
	drainingSource = "node-operation"

	// nodeOperationKind is how an eviction names the interruption that asked
	// for it, in an owner reference.
	nodeOperationKind = "NodeOperation"
)
