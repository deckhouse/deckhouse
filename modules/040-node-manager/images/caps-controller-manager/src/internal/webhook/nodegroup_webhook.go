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

// Package webhook holds CAPS-owned validators for cross-domain resources.
// NodeGroup is one of them: the resource belongs to node-manager, but the
// validity of spec.staticInstances.labelSelector depends on CAPS's own
// state (StaticMachine / StaticInstance), so the check lives next to the
// code that would be surprised by a bad edit.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// MachineNamespace is the namespace where CAPS StaticMachines and the
	// node-controller StaticInstance consumers live.
	MachineNamespace = "d8-cloud-instance-manager"

	// NodeGroupLabel pins a StaticMachine (and the Machine that owns it) to
	// a particular NodeGroup — the same label node-controller sets when it
	// renders the MachineDeployment.
	NodeGroupLabel = "node-group"

	nodeTypeStatic      = "Static"
	nodeTypeCloudStatic = "CloudStatic"
)

var (
	staticMachineListGVK  = schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1alpha1", Kind: "StaticMachineList"}
	staticInstanceListGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstanceList"}

	nodegrouplog = logf.Log.WithName("nodegroup-webhook")
)

// NodeGroupValidator validates NodeGroup admissions that depend on CAPS state.
// It currently enforces one rule: spec.staticInstances.labelSelector cannot be
// introduced on a NodeGroup that has bound StaticInstances outside the new
// selector. See validateLabelSelectorImmutability for the full rationale.
//
// Reader is a live API reader (mgr.GetAPIReader()). The cached client would
// try to spin up an informer for the unstructured GVKs we query and block the
// webhook on an informer sync — not safe during admission.
type NodeGroupValidator struct {
	Reader client.Reader
}

// SetupWithManager registers the validator on the webhook server under the
// path the ValidatingWebhookConfiguration points at.
func (v *NodeGroupValidator) SetupWithManager(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register("/validate-deckhouse-io-v1-nodegroup", &webhook.Admission{
		Handler: v,
	})
}

// nodeGroupPayload is the subset of NodeGroup we read. Avoids importing
// node-manager API types into CAPS.
type nodeGroupPayload struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		NodeType        string           `json:"nodeType"`
		StaticInstances *staticInstances `json:"staticInstances,omitempty"`
	} `json:"spec"`
}

type staticInstances struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// Handle implements admission.Handler. It is scoped tightly: it only acts on
// UPDATE for Static / CloudStatic NodeGroups, and only to run the
// labelSelector guard. Every other NodeGroup validation (CRI, zones, taints,
// etc.) remains in the node-manager admission webhook.
func (v *NodeGroupValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	nodegrouplog.Info("validating nodegroup", "name", req.Name, "operation", req.Operation)

	if req.Operation != "UPDATE" {
		return admission.Allowed("")
	}

	var newNG nodeGroupPayload
	if err := json.Unmarshal(req.Object.Raw, &newNG); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode NodeGroup object: %w", err))
	}

	if newNG.Spec.NodeType != nodeTypeStatic && newNG.Spec.NodeType != nodeTypeCloudStatic {
		return admission.Allowed("")
	}

	var oldNG nodeGroupPayload
	if len(req.OldObject.Raw) > 0 {
		if err := json.Unmarshal(req.OldObject.Raw, &oldNG); err != nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode old NodeGroup object: %w", err))
		}
	}

	denyMsg, err := v.validateLabelSelectorImmutability(ctx, &oldNG, &newNG)
	if err != nil {
		nodegrouplog.Error(err, "failed to validate labelSelector")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if denyMsg != "" {
		return admission.Denied(denyMsg)
	}
	return admission.Allowed("")
}

// validateLabelSelectorImmutability enforces two rules on
// spec.staticInstances.labelSelector:
//
//  1. Once set, it cannot be modified or removed. Reason: the selector is
//     what ties StaticInstances to a NodeGroup; changing it silently
//     reassigns hosts. To change it, the user must create a new NodeGroup.
//  2. Adding a non-empty selector to a NodeGroup that already has bound
//     StaticInstances is refused unless every bound StaticInstance's labels
//     already satisfy the new selector. Reason: the selector reaches
//     StaticMachines via a fresh StaticMachineTemplate applied with the
//     same name, which never triggers a rollout — existing StaticMachines
//     keep the selector they were created with, and their bound
//     StaticInstances keep working as is. Deckhouse does not automatically
//     re-roll static nodes, so the group silently splits into "old SIs
//     without the labels" and "new SIs with them": any future replacement
//     machine applies the new selector and cannot re-adopt the old SIs,
//     so the NodeGroup loses capacity the moment an operator recreates a
//     node. This check forces the operator to label every bound
//     StaticInstance before enabling the selector, so the group stays
//     consistent.
//
// Returns a deny message on rule violation (empty string means allow) or an
// error on transient failures reading cluster state.
func (v *NodeGroupValidator) validateLabelSelectorImmutability(ctx context.Context, oldNG, newNG *nodeGroupPayload) (string, error) {
	oldLS := payloadLabelSelector(oldNG)
	newLS := payloadLabelSelector(newNG)

	if oldLS == nil || labelSelectorIsEmpty(oldLS) {
		// Old selector matched every StaticInstance. If the new one is also
		// empty/absent, nothing to check.
		if newLS == nil || labelSelectorIsEmpty(newLS) {
			return "", nil
		}
		// A non-empty selector is being introduced. Refuse if any currently
		// bound StaticInstance would fall outside the new selector.
		unmatched, err := v.staticInstancesNotMatching(ctx, newNG.Metadata.Name, newLS)
		if err != nil {
			return "", fmt.Errorf("check bound StaticInstances of NodeGroup %s: %w", newNG.Metadata.Name, err)
		}
		if len(unmatched) > 0 {
			return fmt.Sprintf(
				"it is forbidden to add .spec.staticInstances.labelSelector to NodeGroup %q while bound StaticInstances fall outside the new selector: %s. "+
					"These StaticInstances will keep working as they are, but the NodeGroup will be inconsistent: any future replacement machine applies the new selector and cannot re-adopt them, and Deckhouse does not automatically re-roll static nodes. "+
					"Label the listed StaticInstances to match the selector first, then enable it on the NodeGroup",
				newNG.Metadata.Name, strings.Join(unmatched, ", ")), nil
		}
		return "", nil
	}

	// Old selector is non-empty — it is now immutable.
	if newLS == nil || labelSelectorIsEmpty(newLS) {
		return ".spec.staticInstances.labelSelector can be added but cannot be modified or removed once set. To change it, create a new NodeGroup", nil
	}
	if !reflect.DeepEqual(oldLS, newLS) {
		return ".spec.staticInstances.labelSelector can be added but cannot be modified once set. To change it, create a new NodeGroup", nil
	}
	return "", nil
}

func payloadLabelSelector(ng *nodeGroupPayload) *metav1.LabelSelector {
	if ng == nil || ng.Spec.StaticInstances == nil {
		return nil
	}
	return ng.Spec.StaticInstances.LabelSelector
}

func labelSelectorIsEmpty(ls *metav1.LabelSelector) bool {
	return len(ls.MatchLabels) == 0 && len(ls.MatchExpressions) == 0
}

// staticInstancesNotMatching returns the names of StaticInstances that are
// bound (via StaticMachine.status.machineRef) to the given NodeGroup but
// whose labels do not satisfy the given selector. NodeGroup membership is
// resolved through StaticMachines in the d8-cloud-instance-manager namespace
// carrying the "node-group" label — the same identifier CAPS uses when
// reconciling.
func (v *NodeGroupValidator) staticInstancesNotMatching(ctx context.Context, ngName string, ls *metav1.LabelSelector) ([]string, error) {
	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, fmt.Errorf("parse labelSelector: %w", err)
	}

	smList := &unstructured.UnstructuredList{}
	smList.SetGroupVersionKind(staticMachineListGVK)
	nodegrouplog.Info("listing StaticMachines", "namespace", MachineNamespace, "nodeGroup", ngName)
	if err := v.Reader.List(ctx, smList,
		client.InNamespace(MachineNamespace),
		client.MatchingLabels{NodeGroupLabel: ngName},
	); err != nil {
		return nil, fmt.Errorf("list StaticMachines: %w", err)
	}
	if len(smList.Items) == 0 {
		return nil, nil
	}
	smNames := make(map[string]struct{}, len(smList.Items))
	for _, sm := range smList.Items {
		smNames[sm.GetName()] = struct{}{}
	}

	siList := &unstructured.UnstructuredList{}
	siList.SetGroupVersionKind(staticInstanceListGVK)
	nodegrouplog.Info("listing StaticInstances")
	if err := v.Reader.List(ctx, siList); err != nil {
		return nil, fmt.Errorf("list StaticInstances: %w", err)
	}
	var unmatched []string
	for _, si := range siList.Items {
		mrName, found, err := unstructured.NestedString(si.Object, "status", "machineRef", "name")
		if err != nil || !found || mrName == "" {
			continue
		}
		// namespace, when set, must be the CAPS namespace — guard against a
		// same-name collision from an unrelated namespace.
		if mrNs, nsFound, _ := unstructured.NestedString(si.Object, "status", "machineRef", "namespace"); nsFound && mrNs != "" && mrNs != MachineNamespace {
			continue
		}
		if _, bound := smNames[mrName]; !bound {
			continue
		}
		if !selector.Matches(labels.Set(si.GetLabels())) {
			unmatched = append(unmatched, si.GetName())
		}
	}
	sort.Strings(unmatched)
	return unmatched, nil
}
