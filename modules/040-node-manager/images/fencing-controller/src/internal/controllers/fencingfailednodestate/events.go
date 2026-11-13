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

package fencingfailednodestate

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

// eventPrefix starts the message of each event of the controller.
const eventPrefix = "[fencing] "

// announce publishes the same event on the incident and on its Node.
func (r *Reconciler) announce(incident *v1alpha1.FencingFailedNodeState, eventType, reason, format string, args ...any) {
	message := eventPrefix + fmt.Sprintf(format, args...)

	r.recorder.Event(incident, eventType, reason, message)
	r.recorder.Event(&corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       incident.Name,
		// kubectl describe node finds the events by the UID that is equal to
		// the name, as the kubelet writes them.
		UID: types.UID(incident.Name),
	}, eventType, reason, message)
}

// announcePhase publishes the change of the phase and why, if why is not
// empty. An empty phase is the same as Suspected, so the first write of
// Suspected is not a change.
func (r *Reconciler) announcePhase(incident *v1alpha1.FencingFailedNodeState, from, to v1alpha1.FencingFailedNodeStatePhase, why string) {
	if from == "" {
		from = v1alpha1.PhaseSuspected
	}

	if from == to {
		return
	}

	message := fmt.Sprintf("Phase of the fencing incident of the node changed from %s to %s.", from, to)
	if why != "" {
		message += " " + why
	}

	r.announce(incident, corev1.EventTypeNormal, common.ReasonPhaseChanged, "%s", message)
}

func (r *Reconciler) announceTaintSet(incident *v1alpha1.FencingFailedNodeState) {
	r.announce(incident, corev1.EventTypeWarning, common.ReasonOutOfServiceTaintSet,
		"The out-of-service taint is set on the node %s. The pods of the node move to other nodes.", incident.Name)
}

func (r *Reconciler) announceTaintRemoved(incident *v1alpha1.FencingFailedNodeState) {
	r.announce(incident, corev1.EventTypeNormal, common.ReasonOutOfServiceTaintRemoved,
		"The out-of-service taint is removed from the node %s.", incident.Name)
}

func (r *Reconciler) announceNotProgressing(incident *v1alpha1.FencingFailedNodeState, reason, message string) {
	r.announce(incident, corev1.EventTypeWarning, common.ReasonEvictionNotProgressing,
		"The eviction of the node %s is not complete after %s. Reason %s: %s", incident.Name, evictionTimeout, reason, message)
}
