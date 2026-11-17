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

package dexauthenticator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/controller"
)

const (
	// ConditionServed tells whether the shared dex-authenticator instance serves the object on
	// every domain it names, and its reason why it does not.
	ConditionServed = "Served"

	actionReconcile = "Reconcile"

	// maxMessageLength keeps a message within what an Event takes.
	maxMessageLength = 1000
)

// setServed writes the Served condition of the object and, when the condition changed to a
// refusal, an Event with the reason. obj is not changed: it is the object of the cache.
func (r *Reconciler) setServed(ctx context.Context, obj *unstructured.Unstructured, status metav1.ConditionStatus, reason, message string) error {
	if len(message) > maxMessageLength {
		message = message[:maxMessageLength]
	}

	conditions := controller.ReadConditions(obj)
	previous := meta.FindStatusCondition(conditions, ConditionServed)
	wasReason := ""
	if previous != nil {
		wasReason = previous.Reason
	}
	changed := meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:               ConditionServed,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: obj.GetGeneration(),
	})
	if !changed {
		return nil
	}

	updated := obj.DeepCopy()
	if err := unstructured.SetNestedSlice(updated.Object, controller.ConditionItems(conditions), "status", "conditions"); err != nil {
		return fmt.Errorf("set the conditions of DexAuthenticator %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	if err := r.client.Status().Update(ctx, updated); err != nil {
		return fmt.Errorf("update the status of DexAuthenticator %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}

	// Waiting for the instance is how every object starts; it is not worth an Event.
	if r.recorder != nil && status != metav1.ConditionTrue && reason != wasReason && reason != ReasonWaitingForInstance {
		r.recorder.Eventf(obj, nil, corev1.EventTypeWarning, reason, actionReconcile, "%s", message)
	}
	return nil
}
