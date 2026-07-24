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

	// conditionProgress is the single condition an operation carries: where it
	// is and why. The phase says what happened; this says who decided so.
	conditionProgress = "Progress"

	// operationTimeout bounds how long an operation may wait for the node to
	// carry it out. A node that says nothing for this long is not coming back
	// on its own, and an operation left open keeps it out of the scheduler.
	operationTimeout = 30 * time.Minute

	// operationNodeLabel names the node an operation is for, so the operations
	// of one node can be found without reading everyone else's.
	operationNodeLabel = "node-manager.deckhouse.io/node"

	// drainingSource marks the drains this controller asked for, so it only
	// releases nodes it took away itself.
	drainingSource = "node-operation"
)
