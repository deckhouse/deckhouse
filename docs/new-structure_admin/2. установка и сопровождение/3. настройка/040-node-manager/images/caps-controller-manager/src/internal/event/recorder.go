/*
Copyright 2023 Flant JSC

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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
func (r *Recorder) SendNormalEvent(object runtime.Object, nodeGroup string, reason string, message string) {
	r.sendEvent(object, corev1.EventTypeNormal, nodeGroup, reason, message)
}

// SendWarningEvent constructs an event from the given information and puts it in the queue for sending.
func (r *Recorder) SendWarningEvent(object runtime.Object, nodeGroup string, reason string, message string) {
	r.sendEvent(object, corev1.EventTypeWarning, nodeGroup, reason, message)
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

		reportingController := "deckhouse.io/cluster-api-provider-static"

		hostname, err := os.Hostname()
		if err != nil {
			r.logger.Error(err, "Could not get hostname")

			return
		}

		reportingInstance := reportingController + "-" + hostname

		now := time.Now()

		nodeGroupRef, found, err := r.getNodeGroupRef(nodeGroupName)
		if err != nil {
			r.logger.Error(err, "Could not get node group reference", "nodeGroupName", nodeGroupName)

			return
		}

		if found {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = r.client.Create(ctx, makeNodeGroupEvent(objectRef, nodeGroupRef, eventType, reason, message, now, reportingController, reportingInstance))
			if err != nil {
				r.logger.Error(err, "Could not create event for NodeGroup", "object", objectRef, "eventType", eventType, "nodeGroupName", nodeGroupName, "reason", reason, "message", message)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = r.client.Create(ctx, makeObjectEvent(objectRef, nodeGroupRef, eventType, reason, message, now, reportingController, reportingInstance))
		if err != nil {
			r.logger.Error(err, "Could not create event", "object", objectRef, "nodeGroupName", eventType, "nodeGroupName", nodeGroupName, "reason", reason, "message", message)
		}
	}()
}

func (r *Recorder) getNodeGroupRef(nodeGroupName string) (*corev1.ObjectReference, bool, error) {
	if nodeGroupName == "" {
		return nil, false, nil
	}

	nodeGroup := new(unstructured.Unstructured)
	nodeGroup.SetAPIVersion("deckhouse.io/v1")
	nodeGroup.SetKind("NodeGroup")
	nodeGroup.SetName(nodeGroupName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := r.client.Get(ctx, client.ObjectKey{Name: nodeGroupName}, nodeGroup)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to get node group")
	}

	nodeGroupRef, err := ref.GetReference(scheme.Scheme, nodeGroup)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to construct reference")
	}

	return nodeGroupRef, true, nil
}

func makeObjectEvent(objectRef *corev1.ObjectReference, nodeGroupRef *corev1.ObjectReference, eventType string, reason string, msg string, now time.Time, reportingController string, reportingInstance string) *eventsv1.Event {
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
		},
		Regarding:           *objectRef,
		Related:             nodeGroupRef,
		Reason:              reason,
		Note:                msg,
		Type:                eventType,
		EventTime:           metav1.MicroTime{Time: now},
		Action:              "Reconcile",
		ReportingController: reportingController,
		ReportingInstance:   reportingInstance,
	}
}

func makeNodeGroupEvent(objectRef *corev1.ObjectReference, nodeGroupRef *corev1.ObjectReference, eventType string, reason string, msg string, now time.Time, reportingController string, reportingInstance string) *eventsv1.Event {
	return &eventsv1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			// Namespace field has to be filled - event will not be created without it
			// and we have to set 'default' value here for linking this event with a NodeGroup object, which is global
			// if we set 'd8-cloud-instance-manager' here for example, `Events` field on `kubectl describe ng $X` will be empty
			Namespace:    "default",
			GenerateName: "ng-" + nodeGroupRef.Name + "-",
		},
		Regarding:           *nodeGroupRef,
		Related:             objectRef,
		Reason:              reason,
		Note:                msg,
		Type:                eventType,
		EventTime:           metav1.MicroTime{Time: now},
		Action:              "Reconcile",
		ReportingController: reportingController,
		ReportingInstance:   reportingInstance,
	}
}
