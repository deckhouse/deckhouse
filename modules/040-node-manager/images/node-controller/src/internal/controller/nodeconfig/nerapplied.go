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
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nerOutcome is what the nodes report back about one request's sysext.
type nerOutcome struct {
	applied int32
	failed  int32
	// message is one node's account of the refusal. A bad image fails the same
	// way on every node, so the first one is representative; keeping all of them
	// would be a list of identical strings as long as the cluster.
	message string
}

// readNEROutcomes asks the nodes what became of each request, joining
// spec.extensions[].requestedBy against status.extensions[].name; counts are of
// nodes. A node with no status yet counts as neither applied nor failed.
func readNEROutcomes(ctx context.Context, reader client.Reader) (map[string]nerOutcome, error) {
	configs := &internalv1alpha1.NodeConfigList{}
	if err := reader.List(ctx, configs); err != nil {
		return nil, fmt.Errorf("list NodeConfigs: %w", err)
	}

	outcomes := map[string]nerOutcome{}
	for i := range configs.Items {
		config := &configs.Items[i]

		// requestedBy per extension name, for this node.
		owner := make(map[string]string, len(config.Spec.Extensions))
		for _, ext := range config.Spec.Extensions {
			// Only extensions a request put there have a request to report to:
			// platform ones carry the module marker instead, and counting those
			// would invent an owner that does not exist.
			name, ok := strings.CutPrefix(ext.RequestedBy, nerRequestedByPrefix)
			if !ok || name == "" {
				continue
			}
			owner[ext.Name] = name
		}
		if len(owner) == 0 {
			continue
		}

		reported := make(map[string]bool, len(config.Status.Extensions))
		for _, status := range config.Status.Extensions {
			ner, ok := owner[status.Name]
			if !ok {
				continue
			}
			outcome := outcomes[ner]
			switch status.State {
			case extensionStateReady:
				outcome.applied++
				reported[status.Name] = true
			case extensionStateFailed:
				outcome.failed++
				reported[status.Name] = true
				if outcome.message == "" {
					outcome.message = status.Message
				}
			}
			outcomes[ner] = outcome
		}

		// A wholesale rejection leaves only the ConfigurationApplied condition.
		// Two guards: the generation (a stale False says nothing) and the message
		// naming the extension itself (the condition goes False for other reasons).
		cond := meta.FindStatusCondition(config.Status.Conditions, configurationAppliedCondition)
		if cond == nil || cond.Status != metav1.ConditionFalse {
			continue
		}
		if cond.ObservedGeneration != config.Generation {
			continue
		}
		named := extensionNamesIn(cond.Message)
		for name, ner := range owner {
			if reported[name] || !slices.Contains(named, name) {
				continue
			}
			outcome := outcomes[ner]
			outcome.failed++
			if outcome.message == "" {
				outcome.message = cond.Message
			}
			outcomes[ner] = outcome
		}
	}
	return outcomes, nil
}

// extensionNamesIn cuts a node's message into the tokens that can be extension
// names — the CRD allows [a-z0-9-] and nothing else, and the node names the
// extension in prose. Substring search blamed "bob" for refusing "bob-signed".
func extensionNamesIn(message string) []string {
	return strings.FieldsFunc(message, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
	})
}
