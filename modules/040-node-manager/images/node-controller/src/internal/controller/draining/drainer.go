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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kubedrain "github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
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
	// reports holds what the drain running on each node tells about itself.
	reports map[string]*drainReport
}

func newDrainer(ctx context.Context, kubeClient kubernetes.Interface) *drainer {
	return &drainer{
		tasks:      task.NewManager(),
		kubeClient: kubeClient,
		wake:       make(chan event.GenericEvent, wakeBuffer),
		ctx:        ctx,
		reports:    map[string]*drainReport{},
	}
}

// drainReport is what one drain tells about itself: the pods it evicts, once the
// helper lists them, and its ErrOut, where the helper then writes only a line per
// refused eviction, again on every retry.
type drainReport struct {
	mu       sync.Mutex
	pods     []corev1.Pod
	lastSeen map[string]time.Time
}

func (l *drainReport) listed(pods []corev1.Pod) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pods = pods
}

func (l *drainReport) evicting() []corev1.Pod {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pods
}

func (l *drainReport) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastSeen[line] = time.Now()
	return len(p), nil
}

// current returns the refusals repeated within the window, sorted so that an
// unchanged set reads the same.
func (l *drainReport) current(now time.Time) []string {
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
	report := &drainReport{lastSeen: map[string]time.Time{}}
	err := d.tasks.Start(d.ctx, task.TaskID(nodeName),
		func(ctx context.Context) error { return d.evict(ctx, nodeName, timeout, report) },
		func(ctx context.Context) { d.wakeNode(ctx, nodeName) },
	)
	if errors.Is(err, task.ErrExists) {
		return nil
	}
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.reports[nodeName] = report
	d.mu.Unlock()
	logger.Info("drain started")
	return nil
}

// result collects a finished drain, and forgets what it told about itself.
func (d *drainer) result(nodeName string) (bool, error) {
	finished, err := d.tasks.Result(task.TaskID(nodeName))
	if finished {
		d.forget(nodeName)
	}
	return finished, err
}

func (d *drainer) cancel(ctx context.Context, nodeName string) (bool, error) {
	d.forget(nodeName)
	return d.tasks.Cancel(ctx, task.TaskID(nodeName))
}

func (d *drainer) reportOf(nodeName string) *drainReport {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reports[nodeName]
}

// refusals returns the evictions refused within the window to the drain running
// on the node; none once it is collected or cancelled.
func (d *drainer) refusals(nodeName string, now time.Time) []string {
	report := d.reportOf(nodeName)
	if report == nil {
		return nil
	}
	return report.current(now)
}

// progress counts the pods the drain running on the node listed that are still
// there, matched by UID as the helper's waitForDelete does. Nil until the helper
// lists them, and once the drain is collected or cancelled.
func (d *drainer) progress(ctx context.Context, nodeName string) (*deckhousev1alpha1.NodeOperationEviction, error) {
	report := d.reportOf(nodeName)
	if report == nil {
		return nil, nil
	}
	listed := report.evicting()
	if listed == nil {
		return nil, nil
	}

	onNode, err := d.kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("list the pods of %s: %w", nodeName, err)
	}
	current := make(map[types.UID]*corev1.Pod, len(onNode.Items))
	for i := range onNode.Items {
		current[onNode.Items[i].UID] = &onNode.Items[i]
	}

	eviction := &deckhousev1alpha1.NodeOperationEviction{}
	for _, pod := range listed {
		live, ok := current[pod.UID]
		if !ok {
			continue
		}
		eviction.RemainingPods++
		if live.DeletionTimestamp != nil {
			eviction.TerminatingPods = append(eviction.TerminatingPods, pod.Namespace+"/"+pod.Name)
		}
	}
	slices.Sort(eviction.TerminatingPods)
	return eviction, nil
}

func (d *drainer) forget(nodeName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.reports, nodeName)
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

func (d *drainer) evict(ctx context.Context, nodeName string, timeout time.Duration, report *drainReport) error {
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

	if err := runNodeDrain(helper, nodeName, report); err != nil {
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
func runNodeDrain(helper *kubedrain.Helper, nodeName string, report *drainReport) error {
	list, errs := helper.GetPodsForDeletion(nodeName)
	if errs != nil {
		return errors.Join(errs...)
	}
	pods := list.Pods()
	report.listed(pods)
	helper.ErrOut = report
	return helper.DeleteOrEvictPods(pods)
}
