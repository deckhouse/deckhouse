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

// This file is the bashible half of the static-pod roll-up and is meant to be
// removable: when the last mutable NodeGroup is gone, delete it together with
// the two lines that call readAnnotationOutcomes in nsprstatus.go. Nothing else
// in the package reaches into it, which is why the filtering by NodeGroup lives
// here rather than in a shared loop.
package nodeconfig

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// staticPodsAnnotation is where a bashible node lists the static pods it wrote:
// the names of the objects whose manifests are in /etc/kubernetes/manifests,
// comma-separated. Written by the bashible step and never by this controller;
// the step removes the key rather than writing an empty value, so there is one
// spelling of "nothing here".
const staticPodsAnnotation = "node.deckhouse.io/static-pods"

// readAnnotationOutcomes asks the bashible nodes what became of each static pod.
// They have no NodeConfig: their step writes the manifests and lists what it
// wrote in an annotation, so a name in that list is one applied node.
//
// Nothing here is ever a refusal. The annotation carries what succeeded, and a
// step that failed shows up where every failed bashible step does — in the
// node's configuration checksum. So this source fills applied and leaves failed
// and message alone, and "failedNodes: 0" on a cluster with no Engine nodes
// means "nobody complained", not "nothing went wrong".
//
// Nodes of immutable groups are skipped because they answer through their
// NodeConfig instead. That is not tidiness: systemType changes, and a node moved
// from bashible to Engine keeps the annotation its old step left, so counting it
// here as well would report one node applied twice.
func readAnnotationOutcomes(ctx context.Context, reader client.Reader, immutableGroups []string) (map[string]nsprOutcome, error) {
	nodes := &corev1.NodeList{}
	if err := reader.List(ctx, nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	outcomes := map[string]nsprOutcome{}
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if slices.Contains(immutableGroups, node.Labels[nodecommon.NodeGroupLabel]) {
			continue
		}
		for _, name := range staticPodsWritten(node) {
			outcome := outcomes[name]
			outcome.applied++
			outcomes[name] = outcome
		}
	}
	return outcomes, nil
}

// staticPodsWritten reads the names a bashible node put on disk. Trimmed because
// this is a string contract between two components and neither side validates
// it: the step writes no spaces, but one typed in by hand would silently drop
// that node out of appliedNodes.
func staticPodsWritten(node *corev1.Node) []string {
	written := node.Annotations[staticPodsAnnotation]
	if written == "" {
		return nil
	}
	names := strings.Split(written, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return names
}
