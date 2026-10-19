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

package schedule

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/bundle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/dynamic"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule/version"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// FunctionalOrder is the Order of functional (non-critical) packages: above every critical
	// order, so they are gated until all critical packages converged, and bootstrap-gated.
	FunctionalOrder = 999

	reasonRequirementsKubernetes = "KubernetesRequirementsUnmet"
	reasonRequirementsDeckhouse  = "DeckhouseRequirementsUnmet"
	reasonRequirementsBootstrap  = "BootstrapRequirementsUnmet"

	// On the admission path the message is all the user reads, so it spells out the reason.
	subjectKubernetes = "Kubernetes"
	subjectDeckhouse  = "Deckhouse"

	messageRequirementsBootstrap = "cluster bootstrap is not finished yet"
)

// Scheduler is the graph of packages: their rule chains, the facts their Currents publish, the
// enable intents, and one computation that derives every node's decision, inputs, convergence and
// gate (protocol.go). Every exported method is safe for concurrent use and never blocks on a
// consumer; every method that may change the result computes before it returns, so the last
// computation is always the current state.
type Scheduler struct {
	mu    sync.RWMutex
	nodes map[string]*node

	// intents are the Module.spec.enabled intents by name, kept apart from the nodes: an intent
	// set before the node exists, or kept while it is replaced, still applies.
	intents map[string]bool

	paused  bool // paused until Resume: no computation; facts and intents accumulate.
	stopped bool // stopped is final; the wake channel is closed.

	// enabled are the module nodes the running computation decided enabled so far (enabledBefore);
	// used only inside compute.
	enabled []string

	// pending is the latest unread event per node. Not a channel: a bounded channel would block
	// every caller of the mutex whenever the dispatcher lags; events are levels, so slots lose
	// nothing.
	pending map[string]Event
	wake    chan struct{} // wake has capacity 1: "pending changed since the last Take".

	bundleChecker          bundle.BundleChecker
	dependencyGetter       dependency.Getter // dependencyGetter defaults to dependencyVersion.
	kubeVersionGetter      version.Getter
	deckhouseVersionGetter version.Getter
	bootstrapCondition     condition.Condition
	dynamicGetter          dynamic.Getter // dynamicGetter is consulted only without an intent (intentOf).

	logger *log.Logger
}

// Option configures a Scheduler during construction.
type Option func(*Scheduler)

// WithKubeVersionGetter sets the provider for the current Kubernetes version.
func WithKubeVersionGetter(kubeVersionGetter version.Getter) Option {
	return func(s *Scheduler) {
		s.kubeVersionGetter = kubeVersionGetter
	}
}

// WithDeckhouseVersionGetter sets the provider for the current Deckhouse version.
func WithDeckhouseVersionGetter(deckhouseVersionGetter version.Getter) Option {
	return func(s *Scheduler) {
		s.deckhouseVersionGetter = deckhouseVersionGetter
	}
}

// WithBootstrapCondition sets the predicate that gates functional packages until bootstrap.
func WithBootstrapCondition(cond condition.Condition) Option {
	return func(s *Scheduler) {
		s.bootstrapCondition = cond
	}
}

// WithDependencyGetter replaces the graph as the source of dependency versions.
func WithDependencyGetter(getter dependency.Getter) Option {
	return func(s *Scheduler) {
		s.dependencyGetter = getter
	}
}

// WithDynamicGetter sets the provider of a module's enable signal from global hooks (the
// deprecated "<module>Enabled" values patch), consulted only without an intent from SetIntent:
// Module.spec.enabled first, then the hook signal. It is called under the scheduler's mutex and must not
// call back. Its change reaches no fact, so its owner calls Recheck after it.
func WithDynamicGetter(getter dynamic.Getter) Option {
	return func(s *Scheduler) {
		s.dynamicGetter = getter
	}
}

// WithBundleChecker sets the active edition's bundle check.
func WithBundleChecker(getter bundle.BundleChecker) Option {
	return func(s *Scheduler) {
		s.bundleChecker = getter
	}
}

// NewScheduler creates a paused Scheduler with an empty graph. Nothing is computed until Resume:
// before the boot the gate inputs and the intents are unknown, and an unknown input reads as
// Forbid.
func NewScheduler(logger *log.Logger, opts ...Option) *Scheduler {
	s := &Scheduler{
		nodes:   make(map[string]*node),
		intents: make(map[string]bool),
		paused:  true,
		pending: make(map[string]Event),
		wake:    make(chan struct{}, 1),
		logger:  logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.dependencyGetter == nil {
		s.dependencyGetter = s.dependencyVersion
	}

	return s
}

// Resume resumes computations and computes once. A Resume while running does nothing.
func (s *Scheduler) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.paused {
		return
	}

	s.paused = false
	s.compute()
}

// Recheck computes once (when and why: core.Engine.Recheck). A no-op while paused or stopped.
func (s *Scheduler) Recheck() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.compute()
}

// CheckConstraints checks constraints against the cluster and the graph and simulates adding a
// node named name with them. It returns a *ConstraintError for the first unmet requirement, or
// with ReasonDependencyCycle (wrapping a *CycleError) for a cycle. Read-only; while paused the
// dependency answers from the graph are not meaningful yet.
func (s *Scheduler) CheckConstraints(name string, constraints Constraints) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Admission checks requirements only: the gate rules, not the floor or the intent rules.
	rules := s.gateRules(constraints)

	if d := rule.Resolve(rules...); d.Kind == rule.Forbid {
		return &ConstraintError{Reason: d.Reason, Message: d.Message}
	}

	// A nil *ConstraintError must not become a non-nil error interface.
	if err := s.simulateCycle(name, constraints); err != nil {
		return err
	}

	return nil
}

// CheckDependents reports whether replacing the node named name by a revision at version would
// forbid an enabled package that depends on it: one whose dependency on name has a constraint the
// version does not meet. It returns a *ConstraintError with ReasonDependentRequirementsUnmet naming
// every such dependent, sorted; nil when there is none or version is nil. Read-only.
//
// Why: the dependent's own rule would forbid it at the next computation, which disables it and
// deletes its release, a consequence of another package's update nobody asked for. The old module
// dependency extender refused such a release the same way. Only Dependencies count, as there: an
// optional one too, because an optional dependency that is present must still match. AnyOf and
// NoneOf groups are not checked.
func (s *Scheduler) CheckDependents(name string, version *semver.Version) error {
	if version == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var rejected []string

	for _, n := range s.nodes {
		if n.name == name || !n.enabled() {
			continue
		}

		dep, ok := n.dependencies[name]
		if !ok || dependency.Satisfies(dep.Constraint, version) {
			continue
		}

		rejected = append(rejected, fmt.Sprintf("'%s' requires '%s' %s", n.name, name, dep.Constraint))
	}

	if len(rejected) == 0 {
		return nil
	}

	slices.Sort(rejected)

	return &ConstraintError{
		Reason:  ReasonDependentRequirementsUnmet,
		Message: fmt.Sprintf("version %s is not accepted by enabled dependents: %s", version, strings.Join(rejected, "; ")),
	}
}

// simulateCycle returns a ReasonDependencyCycle *ConstraintError if adding or replacing the node
// named name would close a dependency cycle. s.mu held in either mode.
func (s *Scheduler) simulateCycle(name string, constraints Constraints) *ConstraintError {
	snapshot := make(map[string]*node, len(s.nodes)+1)
	for nodeName, n := range s.nodes {
		if nodeName == name {
			continue
		}

		snapshot[nodeName] = n
	}

	snapshot[name] = &node{
		name:         name,
		order:        constraints.Order,
		dependencies: constraints.Dependencies,
	}

	if _, err := topoSort(snapshot); err != nil {
		var cycle *CycleError
		if errors.As(err, &cycle) {
			return &ConstraintError{Reason: ReasonDependencyCycle, Message: cycle.Error(), cause: cycle}
		}

		return &ConstraintError{Reason: ReasonDependencyCycle, Message: err.Error(), cause: err}
	}

	return nil
}

// AddNode adds nd or replaces the node of the same name, dropping its facts: they describe the
// stopped Current. A node that would close a cycle is rejected (ReasonDependencyCycle) and the
// graph is not changed. It computes either way.
func (s *Scheduler) AddNode(nd Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	defer s.compute()

	// A nil *ConstraintError must not become a non-nil error interface.
	if err := s.simulateCycle(nd.Name, nd.Constraints); err != nil {
		return err
	}

	s.addNode(nd)

	return nil
}

// RemoveNode drops the node and its unread event and computes; unknown names are ignored. The
// intent is kept: it belongs to the configuration.
func (s *Scheduler) RemoveNode(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nodes[name]; !ok {
		return
	}

	delete(s.nodes, name)
	delete(s.pending, name)

	s.compute()
}

// Publish stores the facts of the named node and computes. Facts of another revision are ignored,
// so a stopped Current cannot change its replacement's; equal facts skip the computation. They are
// compared under the read lock first: every Current publishes on every wake, mostly equal facts,
// which must not queue on the write lock behind a computation.
func (s *Scheduler) Publish(name string, f Facts) {
	s.mu.RLock()
	changes := s.changes(name, f)
	s.mu.RUnlock()

	if !changes {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Checked again: a Publish, AddNode or RemoveNode may have run between the two locks.
	if !s.changes(name, f) {
		return
	}

	s.nodes[name].facts = f

	s.compute()
}

// changes reports whether f is of the named node's revision and differs from its facts. Called
// under s.mu, read or write.
func (s *Scheduler) changes(name string, f Facts) bool {
	n, ok := s.nodes[name]

	return ok && f.Version == n.revision && n.facts != f
}

// SetIntent sets the Module.spec.enabled intent of name (nil removes it) and computes. It is kept
// by name, across the node's replacement and removal. The module's dynamic rule reads it (see
// addNode for its precedence); applications and global have no such rule.
func (s *Scheduler) SetIntent(name string, enabled *bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, had := s.intents[name]

	switch {
	case enabled == nil && !had:
		return
	case enabled == nil:
		delete(s.intents, name)
	case had && old == *enabled:
		return
	default:
		s.intents[name] = *enabled
	}

	s.compute()
}

// intentOf is the dynamic rule's getter: the explicit intent, else the global hooks' enable
// signal, else no opinion. Called inside compute under s.mu; it must not lock.
func (s *Scheduler) intentOf(name string) *bool {
	if enabled, ok := s.intents[name]; ok {
		return &enabled
	}

	if s.dynamicGetter != nil {
		return s.dynamicGetter(name)
	}

	return nil
}

// Converged reports whether the scheduler is running and the global node and every module node
// are converged. Applications do not count: a broken user application must not keep Deckhouse
// unready.
func (s *Scheduler) Converged() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.paused || s.stopped {
		return false
	}

	for _, n := range s.nodes {
		if (n.module || n.name == GlobalNode) && !n.converged {
			return false
		}
	}

	return true
}

// compute is the one computation, in topological order, under s.mu:
//  1. decision: every node's rule chain (dependencies first; an enabled script sees the modules
//     decided enabled before it);
//  2. inputs: the decision, the node's own inputs and its subscriptions' Output (global: also the
//     enabled modules);
//  3. converged: disabled, or Applied == inputs;
//  4. gate: enabled and everything it waits for converged;
//  5. an event per node, then a wake.
//
// Each step covers all nodes before the next, because global's inputs need every decision and the
// gates every convergence. Computing all of it atomically is the point: if each Current judged
// its own convergence, a dependent could start before its dependency learned it is no longer
// converged. Getter inputs change with the global values, which Global publishes; the rest
// Recheck covers.
func (s *Scheduler) compute() {
	if s.paused || s.stopped {
		return
	}

	// AddNode rejects cycles, so there is no error here; a cycle slipping through would leave its
	// members out of sorted with stale gates, which hold higher tiers and surface the bug.
	sorted, _ := topoSort(s.nodes)

	s.computeOrdering(sorted)

	s.enabled = s.enabled[:0]

	for _, n := range sorted {
		n.decision = rule.Resolve(n.rules...)

		if n.module && n.enabled() {
			s.enabled = append(s.enabled, n.name)
		}
	}

	modules := s.enabledModules()

	for _, n := range sorted {
		n.inputs = s.inputsOf(n, modules)
		n.converged = !n.enabled() || n.facts.Applied == n.inputs
	}

	unconverged := s.lowestUnconverged()

	for _, n := range sorted {
		n.gate = s.gateOf(n, unconverged)
		s.pending[n.name] = s.eventOf(n, modules)
	}

	select {
	case s.wake <- struct{}{}:
	default: // a wake is pending; the consumer takes this batch with it
	}
}

func (s *Scheduler) eventOf(n *node, modules []EnabledModule) Event {
	ev := Event{
		Name:      n.name,
		Version:   n.revision,
		Enabled:   n.enabled(),
		OwnInputs: n.facts.OwnInputs,
		Inputs:    n.inputs,
		Gate:      n.gate,
	}

	if !ev.Enabled {
		ev.Reason, ev.Message = n.decision.Reason, n.decision.Message
	}

	if n.name == GlobalNode {
		ev.EnabledModules = slices.Clone(modules)
	}

	return ev
}

// enabledBefore is an enabled script's global.enabledModules: the modules decided enabled so far in
// this computation, sorted, as addon-operator gave a script the modules enabled before it in its
// pass. Why not Global's published list: it is empty right after a restart, so a script that needs
// another module (kube-proxy needs a CNI) would disable its module on the first computation and
// delete its release. The list is complete for every dependency of the node and for every node
// sorted before it (lower orders first among the nodes ready at a time). Called inside compute
// under s.mu; it must not lock.
func (s *Scheduler) enabledBefore() []string {
	return slices.Sorted(slices.Values(s.enabled))
}

// dependencyVersion answers dependency rules from the graph: an enabled node's version, else nil.
// Callers hold s.mu; it must not lock.
func (s *Scheduler) dependencyVersion(name string) *semver.Version {
	n, ok := s.nodes[name]
	if !ok || !n.enabled() {
		return nil
	}

	return n.version
}

// computeOrdering keeps the AnyOf members placed before each node as its waits and raises the node
// to its highest predecessor's tier, so an order declared below one cannot deadlock the tier
// barrier. sorted must be topological.
func (s *Scheduler) computeOrdering(sorted []*node) {
	placed := make(map[string]struct{}, len(sorted))
	for _, n := range sorted {
		effective := n.order

		for name := range n.dependencies {
			if dep, ok := s.nodes[name]; ok && dep.effectiveOrder > effective {
				effective = dep.effectiveOrder
			}
		}

		// a member placed after n is absent or its edge closed a cycle, so it is not waited for
		n.anyOfWaits = make(map[string]struct{}, len(n.anyOf))
		for name := range n.anyOf {
			if _, ok := placed[name]; !ok {
				continue
			}

			n.anyOfWaits[name] = struct{}{}
			if member := s.nodes[name]; member.effectiveOrder > effective {
				effective = member.effectiveOrder
			}
		}

		n.effectiveOrder = effective
		placed[n.name] = struct{}{}
	}
}

// enabledModules returns every enabled module node with its attachment, sorted by ID
// (Event.EnabledModules).
func (s *Scheduler) enabledModules() []EnabledModule {
	var modules []EnabledModule

	for _, n := range s.nodes {
		if !n.enabled() || !n.module {
			continue
		}

		modules = append(modules, EnabledModule{ID: n.name + "@" + n.revision, Attachment: n.attachment})
	}

	slices.SortFunc(modules, func(a, b EnabledModule) int { return strings.Compare(a.ID, b.ID) })

	return modules
}

// inputsOf hashes everything a run of n depends on: its decision, its own inputs and the Output of
// every subscribed node (empty while absent). Global's inputs also carry the enabled modules,
// which may bring or drop CRDs.
//
// Why Output and not Applied: subscribers render against what Global publishes, not against Global
// having run. A module's upgrade reruns Global but nobody else unless the published values change;
// enabling or disabling a module changes the published set, so everyone reruns. A Global run that
// fails after its publish step keeps the Output that step wrote, so subscribers rerun against it; a
// failed run cancels no other run, and the barrier holds new ones.
func (s *Scheduler) inputsOf(n *node, modules []EnabledModule) string {
	h := sha256.New()

	fmt.Fprintf(h, "enabled=%t\nownInputs=%s\n", n.enabled(), n.facts.OwnInputs)

	for _, name := range slices.Sorted(maps.Keys(n.subscriptions)) {
		output := ""
		if sub, ok := s.nodes[name]; ok {
			output = sub.facts.Output
		}

		fmt.Fprintf(h, "subscription %s=%s\n", name, output)
	}

	if n.name == GlobalNode {
		for _, m := range modules {
			fmt.Fprintf(h, "module %s\n", m.ID)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// lowestUnconverged is the lowest effective order among the unconverged nodes, every node of the
// graph included; MaxUint when all are converged. The tier rule of gateOf compares against it.
func (s *Scheduler) lowestUnconverged() Order {
	lowest := Order(math.MaxUint)

	for _, n := range s.nodes {
		if !n.converged && n.effectiveOrder < lowest {
			lowest = n.effectiveOrder
		}
	}

	return lowest
}

// gateOf reports whether n may run now: it is enabled and nothing blocks it (blockerOf). A disabled
// node counts as converged, so it holds nobody back.
func (s *Scheduler) gateOf(n *node, unconverged Order) bool {
	return n.enabled() && s.blockerOf(n, unconverged) == ""
}

// blockerOf is what n waits for, "" when nothing: the global node, every node of a lower effective
// tier (none below unconverged, lowestUnconverged; named by n's own order, "critical" for a
// functional node), every dependency present in the graph and every kept AnyOf member must be
// converged.
func (s *Scheduler) blockerOf(n *node, unconverged Order) string {
	// The global barrier is a rule of its own, not a consequence of the order: every node renders
	// against what the global run provides, and a node declared at order 0 would otherwise run next
	// to it.
	if g, ok := s.nodes[GlobalNode]; ok && n.name != GlobalNode && !g.converged {
		return GlobalNode
	}

	if unconverged < n.effectiveOrder {
		if n.effectiveOrder >= FunctionalOrder {
			return "critical"
		}

		return fmt.Sprintf("order %d", n.effectiveOrder)
	}

	for name := range n.dependencies {
		if dep, ok := s.nodes[name]; ok && !dep.converged {
			return "dependency " + name
		}
	}

	for name := range n.anyOfWaits {
		if member, ok := s.nodes[name]; ok && !member.converged {
			return "anyOf " + name
		}
	}

	return ""
}
