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

// Package nodestatus writes the agent state to the own Node: the Fencing
// condition, the annotations and the Events. It is the only writer of the Node.
package nodestatus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/metrics"
	"fencing-agent/internal/domain"
)

const (
	// ProfileAnnotation holds the name of the profile of the node group, or
	// "disabled" when fencing is disabled for the group.
	ProfileAnnotation = "node-manager.deckhouse.io/fencing-profile"
	// CPSourceAnnotation holds the source of the control plane state.
	CPSourceAnnotation = "node-manager.deckhouse.io/fencing-cp-source"

	conditionType = "Fencing"

	retryMin = time.Second
	retryMax = 4 * time.Second
)

// writeTimeout limits one Flush. A request to an API server that does not
// answer must not stop the start of the agent and the next writes. Tests
// replace it.
var writeTimeout = 5 * time.Second

// legacyLabels are the labels of the agent 1.0. The agent 2.0 removes them.
var legacyLabels = []string{
	"node-manager.deckhouse.io/fencing-enabled",
	"node-manager.deckhouse.io/fencing-mode",
}

// Events records the Kubernetes Events on the own Node.
type Events interface {
	Normal(reason, message string)
	Warning(reason, message string)
}

type condition struct {
	reason  domain.Reason
	message string
	// since is the time of the last change of the condition status.
	since time.Time
}

// state is the data that the Writer puts on the Node.
type state struct {
	cond       condition
	profile    string
	cpSource   domain.CPSource
	dropLegacy bool
}

// Writer implements domain.Status. The setters never block on the API: Run
// writes the last state and retries on errors.
type Writer struct {
	client  kubernetes.Interface
	node    string
	events  Events
	metrics *metrics.Metrics
	logger  *log.Logger

	wake chan struct{}

	mu   sync.Mutex
	want state

	// flushMu serializes the writes. done is the state that is on the Node.
	flushMu sync.Mutex
	done    state
}

var _ domain.Status = (*Writer)(nil)

func New(client kubernetes.Interface, node string, events Events, m *metrics.Metrics, logger *log.Logger) *Writer {
	return &Writer{
		client:  client,
		node:    node,
		events:  events,
		metrics: m,
		logger:  logger,
		wake:    make(chan struct{}, 1),
	}
}

func (w *Writer) SetCondition(reason domain.Reason, message string) {
	w.mu.Lock()

	prev := w.want.cond
	if prev.reason == reason && prev.message == message {
		w.mu.Unlock()

		return
	}

	w.want.cond = condition{reason: reason, message: message, since: since(prev, reason)}
	w.mu.Unlock()

	w.logger.Info("fencing condition", "reason", reason, "message", message)

	if prev.reason != reason {
		w.metrics.Condition(reason)
		w.record(reason, message)
	}

	w.signal()
}

// WriteCondition writes the Fencing condition to the Node now, with one
// request and without retries. Only a successful write changes the state of
// the Writer, so a failed write does not come to the Node later.
func (w *Writer) WriteCondition(ctx context.Context, reason domain.Reason, message string) error {
	w.mu.Lock()
	c := condition{reason: reason, message: message, since: since(w.want.cond, reason)}
	w.mu.Unlock()

	if err := w.patchCondition(ctx, c); err != nil {
		return err
	}

	w.SetCondition(reason, message)

	return nil
}

func (w *Writer) SetCPSource(source domain.CPSource) {
	w.mu.Lock()

	if w.want.cpSource == source {
		w.mu.Unlock()

		return
	}

	w.want.cpSource = source
	w.mu.Unlock()

	w.metrics.CPSource(source)
	w.signal()
}

func (w *Writer) SetProfile(name string) {
	w.mu.Lock()

	if w.want.profile == name {
		w.mu.Unlock()

		return
	}

	w.want.profile = name
	w.mu.Unlock()

	w.signal()
}

// RemoveLegacyLabels removes the labels of the agent 1.0 from the Node. It makes
// one write. On an error Run repeats it, so the caller may go on.
func (w *Writer) RemoveLegacyLabels(ctx context.Context) error {
	w.mu.Lock()
	w.want.dropLegacy = true
	w.mu.Unlock()

	w.signal()

	return w.Flush(ctx)
}

// Flush writes the changes that are not on the Node yet, one request for the
// annotations and the labels and one for the condition.
func (w *Writer) Flush(ctx context.Context) error {
	w.flushMu.Lock()
	defer w.flushMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	w.mu.Lock()
	want := w.want
	w.mu.Unlock()

	var errs []error

	if want.profile != w.done.profile || want.cpSource != w.done.cpSource || (want.dropLegacy && !w.done.dropLegacy) {
		if err := w.patchMetadata(ctx, want); err != nil {
			errs = append(errs, err)
		} else {
			w.done.profile, w.done.cpSource, w.done.dropLegacy = want.profile, want.cpSource, want.dropLegacy
		}
	}

	if want.cond != w.done.cond {
		if err := w.patchCondition(ctx, want.cond); err != nil {
			errs = append(errs, err)
		} else {
			w.done.cond = want.cond
		}
	}

	return errors.Join(errs...)
}

// Run writes the state to the Node until ctx ends.
func (w *Writer) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		}

		for delay := retryMin; ; delay = min(2*delay, retryMax) {
			err := w.Flush(ctx)
			if err == nil || ctx.Err() != nil {
				break
			}

			w.logger.Warn("write own node failed, retry", "error", err, "delay", delay)

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

func (w *Writer) patchMetadata(ctx context.Context, want state) error {
	meta := map[string]any{}

	annotations := map[string]any{}
	if want.profile != "" {
		annotations[ProfileAnnotation] = want.profile
	}

	if want.cpSource != "" {
		annotations[CPSourceAnnotation] = string(want.cpSource)
	}

	if len(annotations) > 0 {
		meta["annotations"] = annotations
	}

	if want.dropLegacy && !w.done.dropLegacy {
		labels := map[string]any{}
		for _, label := range legacyLabels {
			labels[label] = nil
		}

		meta["labels"] = labels
	}

	data, err := json.Marshal(map[string]any{"metadata": meta})
	if err != nil {
		return fmt.Errorf("encode node metadata patch: %w", err)
	}

	if _, err := w.client.CoreV1().Nodes().Patch(ctx, w.node, types.MergePatchType, data, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patch node metadata: %w", err)
	}

	return nil
}

func (w *Writer) patchCondition(ctx context.Context, c condition) error {
	status := "False"
	if isTrue(c.reason) {
		status = "True"
	}

	// The strategic merge patch merges the conditions by type, so the other
	// conditions stay as they are. The message is always set to replace the old one.
	data, err := json.Marshal(map[string]any{"status": map[string]any{"conditions": []map[string]any{{
		"type":               conditionType,
		"status":             status,
		"reason":             string(c.reason),
		"message":            c.message,
		"lastTransitionTime": metav1.NewTime(c.since),
	}}}})
	if err != nil {
		return fmt.Errorf("encode node condition patch: %w", err)
	}

	if _, err := w.client.CoreV1().Nodes().Patch(ctx, w.node, types.StrategicMergePatchType, data, metav1.PatchOptions{}, "status"); err != nil {
		return fmt.Errorf("patch node condition: %w", err)
	}

	return nil
}

// record writes the Event of a changed reason.
func (w *Writer) record(reason domain.Reason, message string) {
	if message == "" {
		message = string(reason)
	}

	switch reason {
	case domain.ReasonWatchdogTimeoutMismatch, domain.ReasonWatchdogDeviceMissing, domain.ReasonKernelPanicMismatch,
		domain.ReasonInvalidKey, domain.ReasonNodeIPMismatch:
		w.events.Warning(string(reason), message)
	default:
		w.events.Normal(string(reason), message)
	}
}

func (w *Writer) signal() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func isTrue(reason domain.Reason) bool { return reason == domain.ReasonArmed }

// since returns the time of the last change of the condition status after
// the change from prev to reason.
func since(prev condition, reason domain.Reason) time.Time {
	if prev.reason == "" || isTrue(prev.reason) != isTrue(reason) {
		return time.Now()
	}

	return prev.since
}
