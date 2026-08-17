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

// Package events publishes Kubernetes Events on the agent's own Node, so arming,
// disarming and refusals show up in kubectl describe node, not only in the log.
package events

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const component = "fencing-agent"

type Recorder struct {
	broadcaster record.EventBroadcaster
	recorder    record.EventRecorder
	node        *corev1.ObjectReference
}

func New(client kubernetes.Interface, identity domain.NodeIdentity, logger *log.Logger) *Recorder {
	broadcaster := record.NewBroadcaster()

	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	// client-go reports delivery problems through klog, where nobody sees them.
	broadcaster.StartLogging(func(format string, args ...any) {
		logger.Debug("event broadcaster", "message", fmt.Sprintf(format, args...))
	})

	return &Recorder{
		broadcaster: broadcaster,
		recorder: broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
			Component: component,
			Host:      identity.Name,
		}),
		node: &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       identity.Name,
			UID:        types.UID(identity.UID),
		},
	}
}

func (r *Recorder) Normal(reason, message string) {
	r.recorder.Event(r.node, corev1.EventTypeNormal, reason, message)
}

func (r *Recorder) Warning(reason, message string) {
	r.recorder.Event(r.node, corev1.EventTypeWarning, reason, message)
}

// Shutdown flushes the queued events; it must run before the process exits.
func (r *Recorder) Shutdown() {
	r.broadcaster.Shutdown()
}
