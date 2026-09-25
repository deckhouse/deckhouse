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

package entity

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// nodeGroupTrouble is what a NodeGroup says about why its nodes are not appearing. Everything here
// is already in the NodeGroup's status — node-manager writes the cloud provider's own refusal into
// status.lastMachineFailures — and until now none of it was read back while the wait for those
// nodes spun for half an hour on "Nodes Ready 1 of 3".
type nodeGroupTrouble struct {
	name     string
	messages []string
}

// instanceResource is the Instance the node controller keeps for every machine it is bringing up.
// Its conditions carry what bashible reported — the step that failed and why — which is the only
// account of a node that never becomes Ready.
var instanceResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha2", Resource: "instances"}

// instanceTroubles reports what the Instances of the named NodeGroups say about nodes that are not
// coming up. Like nodeGroupTroubles it is diagnostic, so a read that fails adds nothing rather than
// interrupting a wait.
func instanceTroubles(ctx context.Context, kubeCl *client.KubernetesClient, names []string) []nodeGroupTrouble {
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}

	list, err := kubeCl.Dynamic().Resource(instanceResource).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	troubles := make([]nodeGroupTrouble, 0, len(list.Items))
	for _, item := range list.Items {
		// The Instance is labelled with the group it belongs to, the same label the Nodes
		// are selected by above.
		group := item.GetLabels()["node.deckhouse.io/group"]
		if _, ok := wanted[group]; !ok {
			continue
		}

		if message := instanceStatusMessage(item.Object); message != "" {
			troubles = append(troubles, nodeGroupTrouble{name: item.GetName(), messages: []string{message}})
		}
	}

	sort.Slice(troubles, func(i, j int) bool { return troubles[i].name < troubles[j].name })

	return troubles
}

// instanceStatusMessage picks the condition that explains a machine still not carrying a Ready
// node. A condition that is True explains nothing, and neither does one that is merely not there
// yet, so both are passed over.
func instanceStatusMessage(object map[string]any) string {
	conditions, found, err := unstructured.NestedSlice(object, "status", "conditions")
	if err != nil || !found {
		return ""
	}

	// Bashible first: it is the last thing to happen and the thing that most often stops. The
	// machine's own condition only matters when bashible never got the chance to run.
	for _, wanted := range []string{"BashibleReady", "MachineReady"} {
		for _, raw := range conditions {
			condition, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			conditionType, _ := condition["type"].(string)
			status, _ := condition["status"].(string)
			if conditionType != wanted || status == "True" {
				continue
			}

			reason, _ := condition["reason"].(string)
			message, _ := condition["message"].(string)
			return conditionText(conditionType, reason, message)
		}
	}

	return ""
}

func conditionText(conditionType, reason, message string) string {
	text := conditionType
	if reason != "" {
		text += " is " + reason
	} else {
		text += " is not ready"
	}
	if message = strings.TrimSpace(message); message != "" {
		text += ": " + message
	}
	return text
}

// nodeGroupTroubles reports, for each of the named NodeGroups, whatever its status has to say about
// nodes that are not coming up. Groups with nothing to report are left out, so an empty result
// means the NodeGroups themselves are content and the delay is somewhere else.
//
// It is diagnostic: a failure to read the NodeGroups is not worth interrupting a wait over, so
// errors are swallowed and the caller simply gets nothing to add.
func nodeGroupTroubles(ctx context.Context, kubeCl *client.KubernetesClient, names []string) []nodeGroupTrouble {
	list, err := kubeCl.Dynamic().Resource(nodeGroupResource).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}

	troubles := make([]nodeGroupTrouble, 0, len(names))
	for _, item := range list.Items {
		if _, ok := wanted[item.GetName()]; !ok {
			continue
		}

		var ng v1.NodeGroup
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &ng); err != nil {
			continue
		}

		if messages := nodeGroupStatusMessages(ng.Status); len(messages) > 0 {
			troubles = append(troubles, nodeGroupTrouble{name: item.GetName(), messages: messages})
		}
	}

	sort.Slice(troubles, func(i, j int) bool { return troubles[i].name < troubles[j].name })

	return troubles
}

// nodeGroupStatusMessages picks out of a NodeGroup's status the parts that explain missing nodes,
// most specific first: the group's own error, then the machines the provider refused to create,
// then the summary. A group that is simply still working reports nothing.
func nodeGroupStatusMessages(status v1.NodeGroupStatus) []string {
	messages := make([]string, 0, 3)

	if status.Error != "" {
		messages = append(messages, status.Error)
	}

	for _, failure := range status.LastMachineFailures {
		// The provider's refusal — quota exhausted, no capacity in the zone, an image that
		// does not exist — arrives here verbatim, and it is the whole answer.
		description := strings.TrimSpace(failure.LastOperation.Description)
		if description == "" {
			continue
		}

		name := failure.Name
		if name == "" {
			name = "machine"
		}
		messages = append(messages, fmt.Sprintf("%s: %s", name, description))
	}

	// Only worth repeating when nothing more specific was found: on a healthy group it says
	// something bland, and next to a real error it says the same thing twice.
	if len(messages) == 0 && status.ConditionSummary.Ready != "True" && status.ConditionSummary.StatusMessage != "" {
		messages = append(messages, status.ConditionSummary.StatusMessage)
	}

	return messages
}

// describeTroubles renders the troubles for appending to a wait's progress message, under a
// heading naming what they were read from. It returns "" when there is nothing to say, so the
// caller can append it unconditionally.
func describeTroubles(heading string, troubles []nodeGroupTrouble) string {
	if len(troubles) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString(heading)
	for _, trouble := range troubles {
		for _, message := range trouble.messages {
			fmt.Fprintf(&out, "\n* %s | %s", trouble.name, message)
		}
	}

	return out.String()
}

// nodeNotReadyReason returns what kubelet says about a node it is not marking Ready, or "" when it
// says nothing useful.
func nodeNotReadyReason(node corev1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type != corev1.NodeReady {
			continue
		}
		if message := strings.TrimSpace(cond.Message); message != "" {
			return message
		}
		return cond.Reason
	}

	return ""
}
