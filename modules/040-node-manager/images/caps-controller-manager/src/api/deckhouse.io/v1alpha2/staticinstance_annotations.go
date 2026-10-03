/*
Copyright 2025 Flant JSC

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

package v1alpha2

const (
	// SkipBootstrapPhaseAnnotation is an imperative, one-shot request to adopt the node behind
	// spec.address as-is instead of bootstrapping it. CAPS removes the annotation once the node
	// is adopted, and it may only be set while the StaticInstance is in the Pending phase.
	//
	// Because of that it cannot be kept in a declaratively managed manifest: a GitOps-style
	// agent would keep re-adding the annotation that CAPS has just removed, and the update
	// would be rejected as soon as the instance leaves the Pending phase. Use
	// AdoptIfNodeExistsAnnotation for that case.
	SkipBootstrapPhaseAnnotation = "static.node.deckhouse.io/skip-bootstrap-phase"

	// AdoptIfNodeExistsAnnotation is a declarative request to adopt the node behind
	// spec.address if a Node with the same address is already part of the cluster, and to
	// bootstrap it as usual otherwise.
	//
	// Unlike SkipBootstrapPhaseAnnotation it expresses a condition rather than a one-shot
	// action, which makes it idempotent: CAPS neither removes it nor restricts the phase it
	// can be set in, so it may stay in a declaratively managed manifest permanently.
	AdoptIfNodeExistsAnnotation = "static.node.deckhouse.io/adopt-if-node-exists"
)
