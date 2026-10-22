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
	"io"
	"slices"
	"strings"
	"sync"
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

// wakeBuffer sizes the channel carrying finished drains back into the queue.
const wakeBuffer = 128

// refusalWindow is how long a refused eviction counts as still refused. The
// helper retries one every 5s (go_lib/dependency/k8s/drain/drain.go), so a
// refusal it stopped repeating has gone through.
const refusalWindow = 15 * time.Second

// drainer runs one drain per node in the background and hands the node back
// to the workqueue when it is done. It is the only place that knows a drain
// is a goroutine.
type drainer struct {
	tasks      *task.Manager
	kubeClient kubernetes.Interface
	wake       chan event.GenericEvent
	ctx        context.Context

	mu sync.Mutex
	// refused holds what the eviction answered the drain running on each node.
	refused map[string]*refusalLog
}

func newDrainer(ctx context.Context, kubeClient kubernetes.Interface) *drainer {
	return &drainer{
		tasks:      task.NewManager(),
		kubeClient: kubeClient,
		wake:       make(chan event.GenericEvent, wakeBuffer),
		ctx:        ctx,
		refused:    map[string]*refusalLog{},
	}
}

// refusalLog is the ErrOut of one drain: once the pods to evict are listed, the
// helper writes there only a line per refused eviction, again on every retry.
type refusalLog struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
}

func (l *refusalLog) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastSeen[line] = time.Now()
	return len(p), nil
}

// current returns the refusals repeated within the window, sorted so that an
// unchanged set reads the same.
func (l *refusalLog) current(now time.Time) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var lines []string
	for line, seen := range l.lastSeen {
		if now.Sub(seen) > refusalWindow {
			delete(l.lastSeen, line)
			continue
		}
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return lines
}

func (d *drainer) wakeSource() source.Source {
	return source.Channel(d.wake, &handler.EnqueueRequestForObject{})
}

func (d *drainer) start(logger logr.Logger, nodeName string, timeout time.Duration) error {
	refusals := &refusalLog{lastSeen: map[string]time.Time{}}
	err := d.tasks.Start(d.ctx, task.TaskID(nodeName),
		func(ctx context.Context) error { return d.evict(ctx, nodeName, timeout, refusals) },
		func(ctx context.Context) { d.wakeNode(ctx, nodeName) },
	)
	if errors.Is(err, task.ErrExists) {
		return nil
	}
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.refused[nodeName] = refusals
	d.mu.Unlock()
	logger.Info("drain started")
	return nil
}

// result collects a finished drain, and forgets what its evictions were refused.
func (d *drainer) result(nodeName string) (bool, error) {
	finished, err := d.tasks.Result(task.TaskID(nodeName))
	if finished {
		d.forgetRefusals(nodeName)
	}
	return finished, err
}

func (d *drainer) cancel(ctx context.Context, nodeName string) (bool, error) {
	d.forgetRefusals(nodeName)
	return d.tasks.Cancel(ctx, task.TaskID(nodeName))
}

// refusals returns the evictions refused within the window to the drain running
// on the node; none once it is collected or cancelled.
func (d *drainer) refusals(nodeName string, now time.Time) []string {
	d.mu.Lock()
	refusals := d.refused[nodeName]
	d.mu.Unlock()
	if refusals == nil {
		return nil
	}
	return refusals.current(now)
}

func (d *drainer) forgetRefusals(nodeName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.refused, nodeName)
}

// wakeNode hands the node back to the workqueue now that its drain is over.
// A cancelled one is skipped: whoever cancelled it waited for the goroutine and
// moved on. Canceled specifically, not "context is done" — a deadline still has
// a result to record.
func (d *drainer) wakeNode(ctx context.Context, nodeName string) {
	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}

	// The manager's context, not the drain's: on an expired deadline both
	// branches would be ready and select would drop the send half the time.
	select {
	case d.wake <- event.GenericEvent{Object: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}}:
	case <-d.ctx.Done():
	}
}

func (d *drainer) evict(ctx context.Context, nodeName string, timeout time.Duration, refusals io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	helper := kubedrain.NewDrainer(kubedrain.HelperConfig{
		Client: d.kubeClient,
		Timeout: func() *time.Duration {
			timeout := time.Duration(0) // 0 means infinite; actual timeout is controlled by ctx
			return &timeout
		}(),
	})
	helper.Ctx = ctx

	if err := runNodeDrain(helper, nodeName, refusals); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("drain deadline exceeded after %s: %w", timeout, err)
		}
		return err
	}
	return nil
}

// runNodeDrain is kubedrain.RunNodeDrain (go_lib/dependency/k8s/drain/default.go)
// without its warning about the pods it lists, which went to a discarded ErrOut:
// ErrOut is the refusals now.
func runNodeDrain(helper *kubedrain.Helper, nodeName string, refusals io.Writer) error {
	list, errs := helper.GetPodsForDeletion(nodeName)
	if errs != nil {
		return errors.Join(errs...)
	}
	helper.ErrOut = refusals
	return helper.DeleteOrEvictPods(list.Pods())
}
