/*
Copyright 2025 Flant JSC

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

package event

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record/util"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Recorder knows how to record events and send them to Kubernetes API server.
type Recorder struct {
	client client.Client
	logger logr.Logger
}

// NewRecorder returns a Recorder that can be used to send events to Kubernetes API server.
func NewRecorder(client client.Client, logger logr.Logger) *Recorder {
	return &Recorder{
		client: client,
		logger: logger,
	}
}

// SendNormalEvent constructs an event from the given information and puts it in the queue for sending.
func (r *Recorder) SendNormalEvent(object runtime.Object, nodeGroupName string, reason string, message string) {
	r.sendEvent(object, corev1.EventTypeNormal, nodeGroupName, reason, message)
}

// SendWarningEvent constructs an event from the given information and puts it in the queue for sending.
func (r *Recorder) SendWarningEvent(object runtime.Object, nodeGroupName string, reason string, message string) {
	r.sendEvent(object, corev1.EventTypeWarning, nodeGroupName, reason, message)
}

func (r *Recorder) sendEvent(object runtime.Object, eventType string, nodeGroupName string, reason string, message string) {
	objectRef, err := ref.GetReference(scheme.Scheme, object)
	if err != nil {
		r.logger.Error(err, "Could not construct reference", "object", object)
		return
	}

	if !util.ValidateEventType(eventType) {
		r.logger.Error(nil, "Unsupported event type", "eventType", eventType)
		return
	}

	go func() {
		// NOTE: events should be a non-blocking operation
		reportingController := "deckhouse.io/node-controller"

		hostname, err := os.Hostname()
		if err != nil {
			r.logger.Error(err, "Could not get hostname")
			return
		}

		reportingInstance := reportingController + "-" + hostname
		now := time.Now()

		// Create event for the object
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		event := makeEvent(objectRef, nodeGroupName, eventType, reason, message, now, reportingController, reportingInstance)
		err = r.client.Create(ctx, event)
		if err != nil {
			r.logger.Error(err, "Could not create event", "object", objectRef, "eventType", eventType, "reason", reason)
		}
	}()
}

func makeEvent(objectRef *corev1.ObjectReference, nodeGroupName string, eventType string, reason string, msg string, now time.Time, reportingController string, reportingInstance string) *eventsv1.Event {
	namespace := objectRef.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}

	return &eventsv1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: objectRef.Name + "-",
			Labels: map[string]string{
				"node.deckhouse.io/group": nodeGroupName,
			},
		},
		Regarding:           *objectRef,
		Reason:              reason,
		Note:                msg,
		Type:                eventType,
		EventTime:           metav1.MicroTime{Time: now},
		Action:              "Reconcile",
		ReportingController: reportingController,
		ReportingInstance:   reportingInstance,
	}
}
