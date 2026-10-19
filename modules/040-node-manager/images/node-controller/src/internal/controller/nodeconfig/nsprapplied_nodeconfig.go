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
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nsprOutcomes is what the nodes made of each static pod, keyed by the request's name, which is the
// pod's name in spec.staticPods (manifests, by request name). Written counts at the manifest's digest
// whatever generation the node runs; failureAnswers and nodeRefusal say when a node refused it.
func nsprOutcomes(f fleet, manifests map[string]string) map[string]requestOutcome {
	digests := make(map[string]string, len(manifests))
	for name, manifest := range manifests {
		digests[name] = manifestDigest(manifest)
	}

	outcomes := map[string]requestOutcome{}
	for node, config := range f.configs {
		for _, status := range config.Status.StaticPods {
			manifest, requested := manifests[status.Name]
			if !requested {
				continue
			}
			switch status.State {
			case staticPodStateWritten:
				if status.ManifestDigest == digests[status.Name] {
					outcomeOf(outcomes, status.Name).applied[node] = struct{}{}
				}
			case staticPodStateFailed:
				if failureAnswers(config, status.Name, manifest) {
					outcomeOf(outcomes, status.Name).failed[node] = staticPodFailure(status)
				}
			}
		}

		refusal, refused := nodeRefusal(config, f.approvals[node])
		if !refused {
			continue
		}
		for _, pod := range config.Spec.StaticPods {
			if manifest, requested := manifests[pod.Name]; !requested || pod.Manifest != manifest {
				continue
			}
			if outcome := outcomeOf(outcomes, pod.Name); !answered(outcome, node) {
				outcome.failed[node] = refusal
			}
		}
	}
	return outcomes
}

// failureAnswers reports whether a Failed entry is the node's answer about this manifest: the node
// runs its current generation, which gives the pod this manifest or none at all (RemoveFailed).
func failureAnswers(config *internalv1alpha1.NodeConfig, name, manifest string) bool {
	if config.Status.AppliedGeneration != config.Generation {
		return false
	}
	specified, ok := specManifest(config, name)
	return !ok || specified == manifest
}

// specManifest is the manifest the node's spec gives a static pod, if it gives one.
func specManifest(config *internalv1alpha1.NodeConfig, name string) (string, bool) {
	for _, pod := range config.Spec.StaticPods {
		if pod.Name == name {
			return pod.Manifest, true
		}
	}
	return "", false
}

// manifestDigest names a manifest the way the node reports the one it wrote
// (status.staticPods[].manifestDigest, nodelet internal/controllers/staticpods): sha256 of
// the manifest as published, before $MY_IP is expanded.
func manifestDigest(manifest string) string {
	sum := sha256.Sum256([]byte(manifest))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// generationRefusals are the ConfigurationApplied=False reasons that refuse a generation
// whole (nodelet internal/controllers/controller.go). PartiallyApplied is not one: part of
// the generation is on the node, and its per-item entries say which part.
var generationRefusals = []string{"Rejected", "RolledBackToLastKnownGood", "RollbackFailed"}

// configRefused reports the node's refusal of its current generation as a whole. A config
// held for a disruption approval is waiting, not refused; any other False reason (a restart
// into a new agent build, a bootstrap wait) leaves the answer to the per-item entries.
func configRefused(config *internalv1alpha1.NodeConfig, reasons []string) (string, bool) {
	cond := meta.FindStatusCondition(config.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeConfigurationApplied)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return "", false
	}
	if cond.ObservedGeneration != config.Generation || disruptionRequested(config) {
		return "", false
	}
	if !slices.Contains(reasons, cond.Reason) {
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
