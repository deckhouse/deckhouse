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

package nodeconfig

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/ptr"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// requestOutcome is what the nodes report about one request, NodeExtensionRequest and
// NodeStaticPodRequest alike, by node name: the nodes that run it, and the nodes that
// refused it with the cause each of them gives.
type requestOutcome struct {
	applied map[string]struct{}
	failed  map[string]string
}

// outcomeOf returns the request's outcome, made on first use.
func outcomeOf(outcomes map[string]requestOutcome, request string) requestOutcome {
	outcome, ok := outcomes[request]
	if !ok {
		outcome = requestOutcome{applied: map[string]struct{}{}, failed: map[string]string{}}
		outcomes[request] = outcome
	}
	return outcome
}

// answered reports whether the node has answered the request either way.
func answered(outcome requestOutcome, node string) bool {
	if _, ok := outcome.applied[node]; ok {
		return true
	}
	_, ok := outcome.failed[node]
	return ok
}

// fleet is what a status pass knows about the nodes: each node's NodeConfig, the approvals
// of the nodes waiting to be interrupted, and the rollout slots the renders of this pass
// handed out.
type fleet struct {
	configs   map[string]*internalv1alpha1.NodeConfig
	approvals map[string]approvalState
	rollouts  map[string]*rolloutBudget
}

// readFleet reads every NodeConfig, and the NodeOperations of the nodes that wait for a
// disruption approval or that node-manager gave up on, once per pass and from the cache: a
// failed approval is a node's refusal, a standing one its wait.
func (r *Reconciler) readFleet(ctx context.Context, p *pass) (fleet, error) {
	if p.fleet != nil {
		return *p.fleet, nil
	}
	configs := &internalv1alpha1.NodeConfigList{}
	if err := r.Client.List(ctx, configs); err != nil {
		return fleet{}, fmt.Errorf("list NodeConfigs: %w", err)
	}
	f := fleet{
		configs:   make(map[string]*internalv1alpha1.NodeConfig, len(configs.Items)),
		approvals: map[string]approvalState{},
		rollouts:  p.rollouts,
	}
	for i := range configs.Items {
		config := &configs.Items[i]
		f.configs[config.Name] = config
		if !disruptionRequested(config) && !attemptsExhausted(config) {
			continue
		}
		approvals, err := listApprovals(ctx, r.Client, config)
		if err != nil {
			return fleet{}, err
		}
		f.approvals[config.Name] = approvals
	}
	p.fleet = &f
	return f, nil
}

// nerOutcomes is what the nodes made of each request (digests, by request name), joining
// spec.extensions[].requestedBy to status.extensions[].name: Ready and Failed count only at the
// request's digest, and a node that will not apply the revision carrying it (nodeRefusal) refused it.
func nerOutcomes(f fleet, digests map[string]string) map[string]requestOutcome {
	outcomes := map[string]requestOutcome{}
	for node, config := range f.configs {
		// The request behind each extension of this node's spec. Platform extensions
		// carry no requestedBy and have no request to report to.
		requests := make(map[string]string, len(config.Spec.Extensions))
		for _, ext := range config.Spec.Extensions {
			owner := ext.RequestedBy
			if owner == nil || owner.Kind != internalv1alpha1.ObjectOwnerKindNodeExtensionRequest {
				continue
			}
			requests[ext.Name] = owner.Name
		}

		for _, status := range config.Status.Extensions {
			request, ok := requests[status.Name]
			if !ok || status.Digest != digests[request] {
				continue
			}
			switch status.State {
			case extensionStateReady:
				outcomeOf(outcomes, request).applied[node] = struct{}{}
			case extensionStateFailed:
				outcomeOf(outcomes, request).failed[node] = status.Message
			}
		}

		refusal, refused := nodeRefusal(config, f.approvals[node])
		if !refused {
			continue
		}
		for _, ext := range config.Spec.Extensions {
			request, ok := requests[ext.Name]
			if !ok || ext.Digest != digests[request] {
				continue
			}
			if outcome := outcomeOf(outcomes, request); !answered(outcome, node) {
				outcome.failed[node] = refusal
			}
		}
	}
	return outcomes
}

// nodeRefusal is why a node will not apply the revision it holds until the configuration
// changes: it refused or rolled back that revision, or its approvals failed. Only an approved
// retry turns the failure into a wait: an operator's approval can take days to come.
func nodeRefusal(config *internalv1alpha1.NodeConfig, approvals approvalState) (string, bool) {
	if config.Status.AppliedGeneration < config.Generation {
		if refusal, refused := configRefused(config, generationRefusals); refused {
			return refusal, true
		}
	}
	failed := approvals.lastFailed
	if failed == nil {
		return "", false
	}
	refusal := fmt.Sprintf("NodeOperation %s failed: %s", failed.Name, cmp.Or(failed.Status.Message, failed.Status.Reason))
	if retry := approvals.current; retry != nil {
		if ptr.Deref(retry.Spec.Approved, false) {
			return "", false
		}
		return refusal + fmt.Sprintf(". Attempt %d of %d, NodeOperation %s, waits for an operator to set spec.approved to true",
			approvals.failures+1, nodecommon.MaxDisruptionAttempts, retry.Name), true
	}
	if approvals.failures >= nodecommon.MaxDisruptionAttempts {
		refusal += fmt.Sprintf(". node-manager creates no new NodeOperation after %d failed attempts. Fix the cause and change the configuration to retry",
			approvals.failures)
		return refusal, true
	}
	return refusal + fmt.Sprintf(". Attempt %d of %d is requested at %s",
		approvals.failures+1, nodecommon.MaxDisruptionAttempts, approvals.retryAt().UTC().Format(time.RFC3339)), true
}

// refusalMessage names every node that refused a request with the cause it gives. Nodes
// giving the same cause are named together: a bad image fails alike on every node.
func refusalMessage(failed map[string]string) string {
	byCause := map[string][]string{}
	for node, cause := range failed {
		byCause[cause] = append(byCause[cause], node)
	}
	parts := make([]string, 0, len(byCause))
	for cause, nodes := range byCause {
		slices.Sort(nodes)
		part := "node " + nodes[0]
		if len(nodes) > 1 {
			part = "nodes " + strings.Join(nodes, ", ")
		}
		if cause != "" {
			part += ": " + cause
		}
		parts = append(parts, part)
	}
	slices.Sort(parts)
	return strings.Join(parts, "; ")
}

// maxPendingNodes is how many pending nodes the Ready message names; the rest are counted.
const maxPendingNodes = 5

// pendingNodes lists the selected nodes (matched, sorted) that have not answered a request,
// each with why.
func pendingNodes(f fleet, matched []string, outcome requestOutcome, why func(config *internalv1alpha1.NodeConfig) string) []deckhousev1alpha1.RequestNodeStatus {
	var pending []deckhousev1alpha1.RequestNodeStatus
	for _, node := range matched {
		if answered(outcome, node) {
			continue
		}
		reason := "the node has no NodeConfig yet"
		if config, ok := f.configs[node]; ok {
			reason = why(config)
		}
		pending = append(pending, deckhousev1alpha1.RequestNodeStatus{Name: node, State: deckhousev1alpha1.RequestNodeStatePending, Message: reason})
	}
	return pending
}

// pendingMessage names maxPendingNodes of the pending nodes with why, then how many more.
func pendingMessage(pending []deckhousev1alpha1.RequestNodeStatus) string {
	shown := pending[:min(len(pending), maxPendingNodes)]
	parts := make([]string, 0, len(shown)+1)
	for _, node := range shown {
		parts = append(parts, "node "+node.Name+": "+node.Message)
	}
	if more := len(pending) - len(shown); more > 0 {
		parts = append(parts, fmt.Sprintf("and %d more", more))
	}
	return strings.Join(parts, "; ")
}

// statusNodes is status.nodes of a request: the nodes that refused it by name, then the pending
// ones, MaxStatusNodes of them. Each carries the cause or the wait the Ready message gives it.
func statusNodes(failed map[string]string, pending []deckhousev1alpha1.RequestNodeStatus) []deckhousev1alpha1.RequestNodeStatus {
	var nodes []deckhousev1alpha1.RequestNodeStatus
	for _, node := range slices.Sorted(maps.Keys(failed)) {
		nodes = append(nodes, deckhousev1alpha1.RequestNodeStatus{Name: node, State: deckhousev1alpha1.RequestNodeStateFailed, Message: failed[node]})
	}
	nodes = append(nodes, pending...)
	nodes = nodes[:min(len(nodes), deckhousev1alpha1.MaxStatusNodes)]
	for i := range nodes {
		nodes[i].Message = nodecommon.Truncate(nodes[i].Message, maxConditionMessage)
	}
	return nodes
}

// pendingReason says why a node has not answered a request. carries says whether the node's
// spec holds the request as it stands now; held is the node's own word that it holds the
// request back, or empty.
func pendingReason(f fleet, config *internalv1alpha1.NodeConfig, carries bool, held string) string {
	// A silent agent's status is stale, so nothing else read from it is the reason.
	if last := config.Status.LastReconcileTime; !last.IsZero() && time.Since(last.Time) > nodecommon.NodeAgentSilence {
		return "no report since " + last.UTC().Format(time.RFC3339)
	}
	if !carries {
		budget, ok := f.rollouts[config.Labels[nodecommon.NodeGroupLabel]]
		if ok && !budget.hasSlot(config.Name) {
			return "waiting for a rollout slot held by node(s) " + strings.Join(budget.holders(), ", ")
		}
		return "its NodeConfig does not include the request yet"
	}
	if disruptionRequested(config) {
		return approvalWait(f.approvals[config.Name])
	}
	if held != "" {
		return held
	}
	applied := meta.FindStatusCondition(config.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeConfigurationApplied)
	if applied != nil && applied.Reason == unknownFieldsReason {
		return "the node agent does not support some fields of its NodeConfig: " + applied.Message
	}
	return "the node has not reported it yet"
}

// approvalWait says what a node waiting to be interrupted waits for.
func approvalWait(approvals approvalState) string {
	op := approvals.current
	if op == nil {
		return "waiting for a NodeOperation to approve the disruption"
	}
	if op.Status.Phase == "" {
		return "waiting for approval by NodeOperation " + op.Name
	}
	// A fixed text rather than the operation's message: request statuses are
	// re-rendered on a phase change only (operationPhaseChanged), so a message
	// would go stale.
	if op.Status.Phase == deckhousev1alpha1.NodeOperationPhaseApproved {
		return "NodeOperation " + op.Name + " is approved, the node is being prepared"
	}
	return fmt.Sprintf("waiting for approval by NodeOperation %s in phase %s", op.Name, op.Status.Phase)
}

// carriesNER reports whether the node's spec holds the request at the digest it asks for now.
func carriesNER(config *internalv1alpha1.NodeConfig, ner *deckhousev1alpha1.NodeExtensionRequest) bool {
	for _, ext := range config.Spec.Extensions {
		owner := ext.RequestedBy
		if owner == nil || owner.Kind != internalv1alpha1.ObjectOwnerKindNodeExtensionRequest {
			continue
		}
		if owner.Name == ner.Name {
			return ext.Digest == ner.Spec.Extension.Digest
		}
	}
	return false
}

// nerHeld is the node's own word that it holds the extension back: its entry is Pending at
// the request's digest. When the pass waits, ConfigurationApplied says for what, the update
// window for one.
func nerHeld(config *internalv1alpha1.NodeConfig, ext deckhousev1alpha1.NodeExtension) string {
	for _, status := range config.Status.Extensions {
		if status.Name != ext.Name || status.Digest != ext.Digest {
			continue
		}
		if status.State != extensionStatePending {
			return ""
		}
		held := "the extension is Pending on the node"
		wait := meta.FindStatusCondition(config.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeConfigurationApplied)
		if wait != nil && wait.Reason == configurationPendingReason {
			held += ": " + wait.Message
		}
		return held
	}
	return ""
}
