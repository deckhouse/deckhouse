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

// The migration preflight: what has to be true before a cluster still on the previous
// implementation is moved to this one.
//
// Phase 1 of the migration ADR, whose rule is stated there plainly — without a green preflight
// the migration does not begin. Each check names a way the migration is known to go wrong, and
// each is answered from the cluster rather than assumed:
//
//   - the mode the previous implementation is in. In this build only Unmanaged is answerable: the
//     previous implementation's objects do not render here at all, so a cluster that arrives still
//     in Direct or Proxy has already lost them. Mid-transition counts too — its nodes are being
//     reconfigured right now.
//   - Local. The plan rests on the pull path being reachable from outside the cluster, and a
//     cluster whose registry IS the cluster has no such path: it needs the fallback runbook.
//   - an upstream that answers. Bringing the cluster to Unmanaged points every node straight at
//     the upstream, so an unreachable one turns the documented degradation into an outage.
//   - containerd v1 registry configuration written by the operator.
//     The transition rewrites those files and the ADR says they are carried over by hand, so a
//     cluster holding them must not be migrated silently.
//
// What is deliberately NOT asked is whether the new components' images are already on the nodes.
// The ADR floated pre-staging them, and the previous implementation even had a DaemonSet that kept
// images resident, so a check for it looks natural — but the migration has exactly one path, through
// Unmanaged, and Unmanaged means every node pulls straight from a live upstream with nothing stored
// locally. A reachable upstream is not one way of getting those images, it is the prerequisite for
// being in that state at all; where it holds the images are simply pulled, and where it does not the
// cluster cannot be in Unmanaged to begin with. So a pre-staging check could never pass and would
// never need to: it would report a permanent warning about a mechanism the plan does not use.
//
// Blocking marks the checks under which the migration must not begin at all, as against the ones
// naming work to do first. What it does not mean is that this module will refuse: the refusal that
// matters is the switch gate's, which enforces exactly the mode axis — it hands the cluster over
// only from Unmanaged. The mode check here reports that same axis rather than a second opinion on
// it, so an operator reads one answer in two places instead of two answers.
//
// Reachability deliberately gets no such enforcement. By the time the gate looks, the cluster is
// Unmanaged and the previous implementation has already let go of the pull path; refusing the
// takeover then would leave the cluster with neither implementation rather than with a reachable
// registry. It is blocking for the operator's decision, which is made a phase earlier, and that is
// the only place it can be acted on.
//
// The report is a metric per check plus a line in the log. Not a condition on an object, because
// there is no object that belongs to this: the module's own status describes the module, and this
// describes a cluster's readiness to stop being what it is.
package v2

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/checker"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	// PreflightMetric carries one series per check, so an alert names what is wrong rather
	// than only that something is.
	PreflightMetric      = "d8_registry_migration_preflight"
	preflightMetricGroup = "d8_registry_migration_preflight"

	nodeConfigurationSnapName = "preflight-node-configurations"
	preflightSwitchSnapName   = "preflight-v2-switch"
	// Its own subscription to the same secret the switch gate reads: snapshots belong to the
	// hook that asked for them, so sharing the gate's name would read as empty here.
	preflightLegacyStateSnapName = "preflight-legacy-state"
)

// Names of the checks, so that the metric labels and the tests speak the same words.
const (
	CheckMode              = "mode"
	CheckNotLocal          = "not_local"
	CheckUpstreamReachable = "upstream_reachable"
	CheckNodeConfiguration = "node_registry_configuration"
)

// preflightCheck is one answer, phrased as what is missing.
type preflightCheck struct {
	Name   string
	Passed bool

	// Blocking says the migration must not begin while this is unmet — a statement about the
	// operator's decision, not about what this module will allow. See the package doc.
	Blocking bool

	Detail string
}

// nodeConfiguration is one operator-written node configuration, reduced to the question
// this asks of it.
type nodeConfiguration struct {
	Name              string
	TouchesContainerd bool
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// After the switch gate, which reads the same legacy state: this report describes a
		// migration that gate may already have allowed.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 6},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				// The marker that the handover has happened. Subscribed to because the
				// previous implementation's state secret outlives the migration: without
				// this, a cluster that finished migrating would go on being asked whether
				// it is ready to start.
				Name:       preflightSwitchSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{SwitchSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterSwitchSecret,
			},
			{
				Name:       preflightLegacyStateSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{LegacyStateSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterLegacyState,
			},
			{
				Name:       nodeConfigurationSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "NodeGroupConfiguration",
				// Only what an operator wrote. The platform ships configurations of its own,
				// and a check that counted those would report every cluster as holding
				// registry configuration to carry over — a finding that is never actionable
				// reads the same as one that is.
				//
				// NotIn and not DoesNotExist: NotIn matches an object that has no such label
				// at all, which is what an operator's own configuration usually looks like.
				LabelSelector: &v1.LabelSelector{
					MatchExpressions: []v1.LabelSelectorRequirement{{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpNotIn,
						Values:   []string{"deckhouse"},
					}},
				},
				FilterFunc: filterNodeConfiguration,
			},
		},
	},
	handlePreflight,
)

func filterNodeConfiguration(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	content, found, err := unstructured.NestedString(obj.Object, "spec", "content")
	if err != nil {
		return nil, fmt.Errorf("reading spec.content of %s: %w", obj.GetName(), err)
	}

	// Recognised by what it writes, not by its name: an operator names these anything, and the
	// question is whether the transition will overwrite what they wrote. The transition owns the
	// runtime's registry configuration, so a configuration touching it is one to carry over.
	touches := false
	if found {
		lower := strings.ToLower(content)
		for _, marker := range []string{"/etc/containerd/conf.d", "hosts.toml", "registry.d", "registry.mirrors"} {
			if strings.Contains(lower, marker) {
				touches = true
				break
			}
		}
	}

	return nodeConfiguration{Name: obj.GetName(), TouchesContainerd: touches}, nil
}

func handlePreflight(ctx context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(preflightMetricGroup)

	// A cluster installed on this implementation has no legacy state, and a migrated one has a
	// handover behind it. Either way there is no decision to report, and inventing one would
	// put an operator's attention on a question nobody has to answer.
	checks := readPreflight(ctx, input).report()
	if len(checks) == 0 {
		input.Logger.Debug("there is no migration to check on this cluster")
		return nil
	}

	blocking, failed := 0, make([]string, 0, len(checks))
	for _, check := range checks {
		value := 0.0
		if !check.Passed {
			value = 1
			failed = append(failed, fmt.Sprintf("%s (%s)", check.Name, check.Detail))
			if check.Blocking {
				blocking++
			}
		}

		input.MetricsCollector.Set(PreflightMetric, value,
			map[string]string{
				"check":    check.Name,
				"blocking": fmt.Sprintf("%t", check.Blocking),
				"detail":   check.Detail,
			},
			metrics.WithGroup(preflightMetricGroup))
	}

	switch {
	case blocking > 0:
		input.Logger.Warn("the migration must not start on this cluster",
			"blocking", blocking, "checks", strings.Join(failed, "; "))
	case len(failed) > 0:
		input.Logger.Info("the migration has work to do first",
			"checks", strings.Join(failed, "; "))
	default:
		input.Logger.Info("the migration preflight is green")
	}

	return nil
}

// preflight is everything the checks are answered from.
//
// Collected first and judged after, the way the switch gate is: what this reports is read by an
// operator deciding whether to start an irreversible procedure, so it has to be reproducible from
// a written-down cluster state rather than from whatever a hook happened to see.
type preflight struct {
	// AlreadySwitched reports that the handover has happened. Everything below describes a
	// decision that is then in the past, so nothing is reported at all.
	AlreadySwitched bool

	// HasLegacy reports that the previous implementation has recorded a state at all. A
	// cluster installed on this implementation never has, and has no migration ahead of it.
	HasLegacy bool

	// Legacy is what the previous implementation says about itself.
	Legacy legacyState

	// CheckerReported says the registry checker has run at least once. Without it a Status
	// that reads "not ready" is indistinguishable from one nobody has filled in.
	CheckerReported bool
	CheckerStatus   checker.Status

	// NodeConfigurations is what operators have told the nodes to do.
	NodeConfigurations []nodeConfiguration

	// NodeConfigurationsUnreadable is why they could not be read, when they could not.
	NodeConfigurationsUnreadable error
}

func (p preflight) report() []preflightCheck {
	if !p.HasLegacy || p.AlreadySwitched {
		// Silent rather than green. A migrated cluster keeps the previous implementation's
		// state secret, so the checks would go on being answerable — and would go on
		// answering about an upstream this cluster no longer reaches the same way, telling
		// an operator not to start something that finished.
		return nil
	}

	return []preflightCheck{
		p.checkMode(),
		p.checkNotLocal(),
		p.checkUpstream(),
		p.checkNodeConfigurations(),
	}
}

// checkMode answers whether the cluster arrived here in the one state this build serves.
//
// Which is the answer that depends on where this code runs, and it runs here. The migration ADR
// asks this question a release earlier, on the build the cluster is leaving, and there Direct and
// Proxy are ordinary starting points — "settled, you may begin". On THIS build they are not:
// `registry_legacy_owns_the_cluster` is false, so none of the previous implementation's objects
// render, and a cluster that arrives still in Proxy has had the pull path they served deleted from
// under it. Only Unmanaged, where those objects served nothing to begin with, survives the trip.
//
// So the check is not "is it standing still" but "is it standing where this build can serve it".
// Mid-transition is a stop for the older reason: its nodes are being reconfigured as the question
// is asked, so nothing else answered here describes the moment that matters.
//
// It is not going to the previous release, by decision: this branch merges into main, and the
// clusters still on the previous implementation sit on release channels where backporting a checker
// is expensive. So nobody runs this before upgrading, and the population that sees it is exactly
// the one that arrived here without having been brought to Unmanaged. The advice therefore has to
// be actionable from here, which means the image, not the mode: the discriminator has not flipped
// while this is failing, so returning the previous release restores the objects this build removed.
func (p preflight) checkMode() preflightCheck {
	unmanaged := string(registry_const.ModeUnmanaged)

	switch {
	case p.Legacy.Mode == "":
		return preflightCheck{Name: CheckMode, Blocking: true,
			Detail: "the previous implementation has not recorded a mode yet"}

	case p.Legacy.TargetMode != "" && p.Legacy.TargetMode != p.Legacy.Mode:
		return preflightCheck{Name: CheckMode, Blocking: true,
			Detail: fmt.Sprintf("a transition is in flight: %s to %s", p.Legacy.Mode, p.Legacy.TargetMode)}

	case p.Legacy.Mode != unmanaged:
		return preflightCheck{Name: CheckMode, Blocking: true,
			Detail: fmt.Sprintf("the cluster arrived in %s, and this build renders none of the "+
				"objects that mode needs; return the previous release, bring it to %s there, "+
				"then upgrade again", p.Legacy.Mode, unmanaged)}

	default:
		return preflightCheck{Name: CheckMode, Passed: true,
			Detail: fmt.Sprintf("settled in %s", unmanaged)}
	}
}

// checkNotLocal is the hard stop the ADR spells out.
//
// In Local the cluster's registry is the cluster. Bringing it to Unmanaged points every node at
// an upstream it does not have, so the documented procedure cannot apply and the fallback runbook
// does — adoption of the existing store, by hand, on single clusters.
func (p preflight) checkNotLocal() preflightCheck {
	if p.Legacy.Mode == string(registry_const.ModeLocal) {
		return preflightCheck{Name: CheckNotLocal, Blocking: true,
			Detail: "the cluster is in Local: use the fallback runbook, not this procedure"}
	}
	return preflightCheck{Name: CheckNotLocal, Passed: true, Detail: "not in Local"}
}

// checkUpstream reports what the registry checker already established.
//
// The load-bearing one, now that pre-staging is out: Unmanaged has no local copy of anything, so a
// reachable upstream is the whole pull path of every node for the length of the migration window.
//
// Its own probe is deliberately not opened. The checker asks this question on a schedule and holds
// the answer, while a hook dialling a registry would hold the queue while doing it and answer a
// different moment than the one the migration happens in. And it is fed exactly when this reports:
// `checker.SetParams` is called only by the previous implementation's hook, which runs while the
// current one is held off — the same condition under which there is a migration to check.
func (p preflight) checkUpstream() preflightCheck {
	switch {
	case !p.CheckerReported:
		return preflightCheck{Name: CheckUpstreamReachable, Blocking: true,
			Detail: "the registry checker has not reported yet, so reachability is unknown"}
	case !p.CheckerStatus.Ready:
		detail := p.CheckerStatus.Message
		if detail == "" {
			detail = "the checker reports the registry as not ready"
		}
		return preflightCheck{Name: CheckUpstreamReachable, Blocking: true, Detail: detail}
	default:
		return preflightCheck{Name: CheckUpstreamReachable, Passed: true,
			Detail: "the checker reaches the registry the cluster pulls from"}
	}
}

// checkNodeConfigurations answers whether the operator wrote registry configuration of their own.
//
// The transition owns the container runtime's registry files, so anything an operator put there is
// overwritten by it. The ADR carries those over by hand, which makes this a warning naming the
// configurations rather than a stop: only the operator knows whether the carrying over is done.
func (p preflight) checkNodeConfigurations() preflightCheck {
	if p.NodeConfigurationsUnreadable != nil {
		return preflightCheck{Name: CheckNodeConfiguration,
			Detail: fmt.Sprintf("the node configurations could not be read: %s",
				p.NodeConfigurationsUnreadable.Error())}
	}

	touching := make([]string, 0, len(p.NodeConfigurations))
	for _, configuration := range p.NodeConfigurations {
		if configuration.TouchesContainerd {
			touching = append(touching, configuration.Name)
		}
	}
	sort.Strings(touching)

	if len(touching) > 0 {
		return preflightCheck{Name: CheckNodeConfiguration,
			Detail: fmt.Sprintf("carry over by hand, the transition overwrites what they write: %s",
				strings.Join(touching, ", "))}
	}

	return preflightCheck{Name: CheckNodeConfiguration, Passed: true,
		Detail: "no node configuration writes registry settings"}
}

// readPreflight collects the inputs.
//
// A subject with no checks to report is how "there is no migration here" is expressed, so that the
// two ways of having none — never started, already finished — are decided in one pure place rather
// than in the handler.
func readPreflight(ctx context.Context, input *go_hook.HookInput) preflight {
	if _, err := helpers.SnapshotToSingle[string](input, preflightSwitchSnapName); err == nil {
		return preflight{AlreadySwitched: true}
	}

	legacy, err := helpers.SnapshotToSingle[legacyState](input, preflightLegacyStateSnapName)
	if err != nil {
		return preflight{}
	}

	subject := preflight{
		HasLegacy:       true,
		Legacy:          legacy,
		CheckerReported: checker.Initialized(input),
		CheckerStatus:   checker.GetStatus(ctx, input),
	}

	configurations, err := helpers.SnapshotToList[nodeConfiguration](input, nodeConfigurationSnapName)
	if err != nil {
		subject.NodeConfigurationsUnreadable = err
	} else {
		subject.NodeConfigurations = configurations
	}

	return subject
}
