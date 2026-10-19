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

package common

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// MaxDisruptionAttempts is how many failed Disruption NodeOperations node-manager allows
// for one NodeConfig revision before it gives up. A Completed one never counts: the node
// released it. A retry waiting for approval names the limit.
const MaxDisruptionAttempts = 3

// A failed disruption takes the node out of service and changes nothing, so the
// retries for one configuration revision are bounded: each attempt waits longer
// than the one before, and after MaxDisruptionAttempts the revision is left alone.
const (
	disruptionRetryBackoff    = time.Minute
	disruptionRetryBackoffMax = 10 * time.Minute
)

// NodeOperationRetention is how long a finished NodeOperation is kept: long enough to investigate,
// and the list stops growing. A failed attempt counts against MaxDisruptionAttempts until then.
const NodeOperationRetention = 24 * time.Hour

// DisruptionRetryBackoff is how long node-manager waits after the newest of the given
// number of failed attempts before it creates the next one. The nodeconfig controller
// waits it out, and the nodeoperation controller publishes the time in Retry.
func DisruptionRetryBackoff(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}
	backoff := disruptionRetryBackoff
	for range failures - 1 {
		backoff = min(2*backoff, disruptionRetryBackoffMax)
	}
	return backoff
}

// DisruptionApprovalWithheld returns the reason of a False DisruptionApproved, empty for any
// other status, and whether the condition is about the revision the NodeConfig holds now.
func DisruptionApprovalWithheld(nc *internalv1alpha1.NodeConfig) (string, bool) {
	cond := meta.FindStatusCondition(nc.Status.Conditions, internalv1alpha1.NodeConfigConditionTypeDisruptionApproved)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return "", false
	}
	return cond.Reason, cond.ObservedGeneration == nc.Generation
}
