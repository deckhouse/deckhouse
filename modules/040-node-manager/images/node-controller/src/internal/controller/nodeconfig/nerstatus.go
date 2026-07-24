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
	"sort"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// readyConditionType answers whether a request's sysext resolved to an image the
// nodes can pull.
const readyConditionType = "Ready"

// reconcileNERStatuses writes each NodeExtensionRequest's status: the image its
// sysext resolves to, the immutable NodeGroups it selects, and a Ready condition
// carrying the reason when it does not resolve. It is best-effort — a status
// write failing never blocks node rendering — and runs off the same pass that
// re-renders nodes, so a NER edit refreshes both.
func (r *Reconciler) reconcileNERStatuses(ctx context.Context, logger logr.Logger) {
	ners := &deckhousev1alpha1.NodeExtensionRequestList{}
	if err := r.Client.List(ctx, ners); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.Error(err, "cannot list NodeExtensionRequests for status")
		}
		return
	}
	if len(ners.Items) == 0 {
		return
	}

	groups := r.immutableNodeGroupNames(ctx, logger)

	for i := range ners.Items {
		if err := r.updateNERStatus(ctx, &ners.Items[i], groups); err != nil {
			logger.Error(err, "cannot update NodeExtensionRequest status", "request", ners.Items[i].Name)
		}
	}
}

// immutableNodeGroupNames lists the names of the immutable NodeGroups a request
// can select. A listing failure yields no names — the status simply reports no
// matched groups rather than blocking.
func (r *Reconciler) immutableNodeGroupNames(ctx context.Context, logger logr.Logger) []string {
	ngs := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngs); err != nil {
		logger.Error(err, "cannot list NodeGroups for NodeExtensionRequest status")
		return nil
	}
	var names []string
	for i := range ngs.Items {
		if ngs.Items[i].Spec.SystemType == v1.SystemTypeImmutable {
			names = append(names, ngs.Items[i].Name)
		}
	}
	return names
}

// updateNERStatus computes and patches one request's status, skipping the write
// when nothing changed.
func (r *Reconciler) updateNERStatus(ctx context.Context, ner *deckhousev1alpha1.NodeExtensionRequest, immutableGroups []string) error {
	desired := ner.Status.DeepCopy()
	desired.ObservedGeneration = ner.Generation
	desired.MatchedNodeGroups = matchedNodeGroups(ner, immutableGroups)

	ext, reason := resolveExtension(ner)
	if reason == "" {
		desired.ResolvedImage = fmt.Sprintf("%s/%s@%s", ext.Repository, ext.AdditionalPath, ext.Digest)
		meta.SetStatusCondition(&desired.Conditions, metav1.Condition{
			Type:               readyConditionType,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: ner.Generation,
			Reason:             reasonResolved,
			Message:            "sysext resolved to " + desired.ResolvedImage,
		})
	} else {
		desired.ResolvedImage = ""
		meta.SetStatusCondition(&desired.Conditions, metav1.Condition{
			Type:               readyConditionType,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ner.Generation,
			Reason:             reason,
			Message:            nerReasonMessage(reason),
		})
	}

	if apiequality.Semantic.DeepEqual(&ner.Status, desired) {
		return nil
	}
	patched := ner.DeepCopy()
	patched.Status = *desired
	if err := r.Client.Status().Patch(ctx, patched, client.MergeFrom(ner)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}

// matchedNodeGroups returns the immutable NodeGroups the request selects: the
// listed names it intersects, or all of them when it names none.
func matchedNodeGroups(ner *deckhousev1alpha1.NodeExtensionRequest, immutableGroups []string) []string {
	names := ner.Spec.NodeGroupSelector.MatchNames
	if len(names) == 0 {
		matched := append([]string(nil), immutableGroups...)
		sort.Strings(matched)
		return matched
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	var matched []string
	for _, name := range immutableGroups {
		if _, ok := wanted[name]; ok {
			matched = append(matched, name)
		}
	}
	sort.Strings(matched)
	return matched
}

// nerReasonMessage explains a not-ready reason for the condition message.
func nerReasonMessage(reason string) string {
	switch reason {
	case reasonInvalidSysext:
		return "sysext must set both name and digest"
	case reasonNoPath:
		return "sysext path is unset and no module.deckhouse.io/name label to default it from"
	default:
		return reason
	}
}
