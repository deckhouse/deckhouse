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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nsprOutcome is what the fleet reports back about one static pod. Same shape
// and counting loop as nerOutcome/readNEROutcomes (nerapplied.go); kept apart
// because the join differs.
type nsprOutcome struct {
	applied int32
	failed  int32
	// message is one node's account of the refusal. A bad manifest fails the
	// same way on every node, so the first one is representative; keeping all of
	// them would be a list of identical strings as long as the cluster.
	message string
}

// readNodeConfigOutcomes asks the Engine nodes what became of each static pod.
// The key is the pod's name, which is the NodeStaticPodRequest's own name — the
// render copies it across unchanged, so no other join is needed.
//
// Which nodes are its own is not a question this source has to ask: every node
// of every group has a NodeConfig, and its status.staticPods is the one report.
// Hence no NodeGroups, no labels and no systemType here.
//
// The manifest on disk is the whole of what the node owes the pod, so
// status.staticPods[] is the whole of the answer, and it has two states: Written
// is applied, Failed is refused whatever its reason. A node with no entry has not
// reported and is counted neither way.
//
// Entries count only while the node runs the current generation: a held or
// rolled-back node still reports the manifest it had before. A node refusing
// that generation whole fails every static pod the generation carries.
func readNodeConfigOutcomes(ctx context.Context, reader client.Reader) (map[string]nsprOutcome, error) {
	configs := &internalv1alpha1.NodeConfigList{}
	if err := reader.List(ctx, configs); err != nil {
		return nil, fmt.Errorf("list NodeConfigs: %w", err)
	}

	outcomes := map[string]nsprOutcome{}
	for i := range configs.Items {
		config := &configs.Items[i]
		if refusal, refused := configRefused(config); refused {
			for _, pod := range config.Spec.StaticPods {
				outcome := outcomes[pod.Name]
				outcome.failed++
				if outcome.message == "" {
					outcome.message = refusal
				}
				outcomes[pod.Name] = outcome
			}
			continue
		}
		if config.Status.AppliedGeneration != config.Generation {
			continue
		}
		for _, status := range config.Status.StaticPods {
			outcome := outcomes[status.Name]
			switch status.State {
			case staticPodStateWritten:
				outcome.applied++
			case staticPodStateFailed:
				outcome.failed++
				if outcome.message == "" {
					outcome.message = staticPodFailure(status)
				}
			default:
				continue
			}
			outcomes[status.Name] = outcome
		}
	}
	return outcomes, nil
}

// configRefused reports the node's refusal of its current generation as a whole.
// A config held for a disruption approval is waiting, not refused.
func configRefused(config *internalv1alpha1.NodeConfig) (string, bool) {
	cond := meta.FindStatusCondition(config.Status.Conditions, configurationAppliedCondition)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return "", false
	}
	if cond.ObservedGeneration != config.Generation || disruptionRequested(config) {
		return "", false
	}
	return cond.Reason + ": " + cond.Message, true
}

// staticPodFailure is one node's account of a refusal, reason first. The reason
// is what tells an operator which way to go — ManifestRejected is theirs to fix,
// WriteFailed is the node's — and a message alone has said neither.
func staticPodFailure(status internalv1alpha1.StaticPodStatus) string {
	switch {
	case status.Reason == "":
		return status.Message
	case status.Message == "":
		return status.Reason
	default:
		return status.Reason + ": " + status.Message
	}
}
