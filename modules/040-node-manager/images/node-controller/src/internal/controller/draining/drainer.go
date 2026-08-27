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

package draining

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubedrain "github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"

	"github.com/deckhouse/node-controller/internal/task"
)

// wakeBuffer sizes the channel carrying finished evictions back into the queue.
const wakeBuffer = 128

// errDrainDeadline marks an eviction that ran out of its timeout rather than
// failing. A failure is retried; a deadline is not, because its cause is
// durable — a budget that never allows eviction, a pod that never terminates.
var errDrainDeadline = errors.New("drain deadline exceeded")

// drainer runs one eviction per node in the background and hands the node back
// to the workqueue when it is done. It is the only place that knows an eviction
// is a goroutine.
type drainer struct {
	tasks      *task.Manager
	kubeClient kubernetes.Interface
	wake       chan event.GenericEvent
	parent     context.Context
}

func newDrainer(parent context.Context, kubeClient kubernetes.Interface) *drainer {
	return &drainer{
		tasks:      task.NewManager(),
		kubeClient: kubeClient,
		wake:       make(chan event.GenericEvent, wakeBuffer),
		parent:     parent,
	}
}

func (d *drainer) wakeSource() source.Source {
	return source.Channel(d.wake, &handler.EnqueueRequestForObject{})
}

func (d *drainer) start(logger logr.Logger, nodeName string, timeout time.Duration) error {
	err := d.tasks.Start(d.parent, task.TaskID(nodeName),
		func(ctx context.Context) error { return d.evict(ctx, nodeName, timeout) },
		func(ctx context.Context) { d.wakeNode(ctx, nodeName) },
	)
	if errors.Is(err, task.ErrExists) {
		return nil
	}
	if err != nil {
		return err
	}

	logger.Info("eviction started")
	return nil
}

func (d *drainer) result(nodeName string) (bool, error) {
	return d.tasks.Result(task.TaskID(nodeName))
}

func (d *drainer) cancel(ctx context.Context, nodeName string) (bool, error) {
	return d.tasks.Cancel(ctx, task.TaskID(nodeName))
}

// wakeNode hands the node back to the workqueue now that its eviction is over.
// A cancelled one is skipped: whoever cancelled it waited for the goroutine and
// moved on. Canceled specifically, not "context is done" — a deadline still has
// a result to record.
func (d *drainer) wakeNode(ctx context.Context, nodeName string) {
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}

	// The manager's context, not the eviction's: on an expired deadline both
	// branches would be ready and select would drop the send half the time.
	select {
	case d.wake <- event.GenericEvent{Object: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}}:
	case <-d.parent.Done():
	}
}

func (d *drainer) evict(ctx context.Context, nodeName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	helper := kubedrain.NewDrainer(kubedrain.HelperConfig{
		Client:  d.kubeClient,
		Timeout: &timeout,
	})
	helper.Ctx = ctx

	if err := kubedrain.RunNodeDrain(helper, nodeName); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w after %s: %w", errDrainDeadline, timeout, err)
		}
		return err
	}
	return nil
}
