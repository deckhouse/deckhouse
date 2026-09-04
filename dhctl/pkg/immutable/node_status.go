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

package immutable

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"
)

// NodeStatus is what a node installing itself reports on its maintenance port.
// The shape of the handoff Status plus the conditions, which is what tells a
// node still working from one that will never register.
type NodeStatus struct {
	Phase      Phase           `json:"phase"`
	Message    string          `json:"message,omitempty"`
	Conditions []NodeCondition `json:"conditions,omitempty"`
}

// NodeCondition is one thing the node has decided about itself.
type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ConditionAddressResolved is the node's answer to the question this whole
// contract is about: which of its interfaces the cluster reaches it on. False
// means it never registers, however long anybody waits.
const ConditionAddressResolved = "AddressResolved"

// ConditionFalse is the condition status of a decision the node could not make.
const ConditionFalse = "False"

// Condition returns the named condition, or nil where the node reports none.
func (s *NodeStatus) Condition(conditionType string) *NodeCondition {
	i := slices.IndexFunc(s.Conditions, func(c NodeCondition) bool { return c.Type == conditionType })
	if i < 0 {
		return nil
	}
	return &s.Conditions[i]
}

// FetchNodeStatus asks a machine what it is doing with the document it was
// handed. Authorised by the token in that very document, so only the installer
// that wrote it can read the machine's progress.
func FetchNodeStatus(ctx context.Context, address, token string) (*NodeStatus, error) {
	response, err := do(ctx, http.MethodGet, "http://"+address+nodeStatusPath, token, nil)
	if err != nil {
		return nil, fmt.Errorf("read the bootstrap status of %s: %w", address, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("read the bootstrap status of %s: %s: %s", address, response.Status, errorQuote(response))
	}

	status := &NodeStatus{}
	if err := json.NewDecoder(io.LimitReader(response.Body, inventoryResponseLimit)).Decode(status); err != nil {
		return nil, fmt.Errorf("parse the bootstrap status of %s: %w", address, err)
	}
	return status, nil
}

// StatusTokenOf reads the status bearer out of a NodeConfig dhctl has just
// built, so nothing has to carry it alongside the document it lives in.
func StatusTokenOf(nodeConfigYAML []byte) string {
	var parsed nodeConfig
	if err := yaml.Unmarshal(nodeConfigYAML, &parsed); err != nil {
		return ""
	}
	return parsed.Spec.StatusToken
}

// DescribeNodeStatus is the one line a wait prints about what the node says.
func DescribeNodeStatus(status *NodeStatus) string {
	described := []string{string(status.Phase)}
	if status.Message != "" {
		described = append(described, status.Message)
	}
	for _, condition := range status.Conditions {
		described = append(described, fmt.Sprintf("%s=%s", condition.Type, condition.Status))
	}
	return strings.Join(described, ", ")
}
