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
	"strings"

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

// readNEROutcomes asks the nodes what became of each request.
//
// The controller writes spec.extensions and, until now, never read back what
// happened: a NodeExtensionRequest reported Ready as soon as its sysext
// resolved, which says the image reference is well formed and nothing more. An
// extension every node refuses — the case this exists for is a roothash signed
// by a key the kernel does not trust — looked exactly like one that works.
//
// The join key is spec.extensions[].requestedBy, which already carries the
// request's own name, matched against status.extensions[].name. Both sides are
// per node, so the counts are of nodes, not of extensions.
//
// A node with no status yet counts as neither applied nor failed: it has not
// answered, and reporting silence as either would be inventing an answer.
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

		for _, status := range config.Status.Extensions {
			ner, ok := owner[status.Name]
			if !ok {
				continue
			}
			outcome := outcomes[ner]
			switch status.State {
			case extensionStateReady:
				outcome.applied++
			case extensionStateFailed:
				outcome.failed++
				if outcome.message == "" {
					outcome.message = status.Message
				}
			}
			outcomes[ner] = outcome
		}
	}
	return outcomes, nil
}
