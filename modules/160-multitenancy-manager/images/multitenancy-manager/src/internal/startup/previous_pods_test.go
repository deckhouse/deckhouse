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

package startup

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	testNamespace = "d8-multitenancy-manager"
	testPod       = "multitenancy-manager-new-a"
	testInterval  = 5 * time.Second
	testTimeout   = time.Minute
	testFinalizer = "test.deckhouse.io/hold"
)

var testSelector = map[string]string{"app": "multitenancy-manager"}

func TestPreviousPods_Wait(t *testing.T) {
	tests := []struct {
		name string
		// podName is the name the controller is told it runs as; empty when not set.
		podName string
		pods    []client.Object
		// expectedWait is how long Wait blocks before it returns nil.
		expectedWait time.Duration
		// expectedEvent is the Warning event recorded when Wait gives up; empty when none.
		expectedEvent string
	}{
		{
			name:    "only pods of this revision",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-new-b", "new"),
			},
		},
		{
			name:    "finished pods of another revision are not waited for",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-evicted", "old", inPhase(corev1.PodFailed)),
				newPod("multitenancy-manager-old-completed", "old", inPhase(corev1.PodSucceeded)),
			},
		},
		{
			name:    "pods outside the deployment are not waited for",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-elsewhere", "old", inNamespace("default")),
				newPod("other-app", "old", withLabel("app", "other")),
				newPod("multitenancy-manager-bare", "", withoutLabel(appsv1.DefaultDeploymentUniqueLabelKey)),
			},
		},
		{
			name: "no pod name set",
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old"),
			},
		},
		{
			name:    "this pod belongs to no revision",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "", withoutLabel(appsv1.DefaultDeploymentUniqueLabelKey)),
				newPod("multitenancy-manager-old-a", "old"),
			},
		},
		{
			name:    "running pod of another revision",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old"),
			},
			expectedWait:  testTimeout,
			expectedEvent: "Warning " + ReasonPreviousRevisionStillRunning,
		},
		{
			name:    "terminating pod of another revision",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old", terminating),
			},
			expectedWait:  testTimeout,
			expectedEvent: "Warning " + ReasonPreviousRevisionStillRunning,
		},
		{
			name:    "a pod of an older revision is waited for",
			podName: testPod,
			pods: []client.Object{
				replicaSet("new", 2), replicaSet("old", 1),
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old"),
			},
			expectedWait:  testTimeout,
			expectedEvent: "Warning " + ReasonPreviousRevisionStillRunning,
		},
		{
			// the older pod took the lease first; the newer one cannot write without it
			name:    "a pod of a newer revision is not waited for",
			podName: "multitenancy-manager-old-a",
			pods: []client.Object{
				replicaSet("new", 2), replicaSet("old", 1),
				newPod(testPod, "new", inPhase(corev1.PodPending)),
				newPod("multitenancy-manager-old-a", "old"),
			},
		},
		{
			// the bound runs from the deletion of the pod, not from every start of the controller
			name:    "a pod stuck terminating for longer than the timeout is not waited for",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old", terminatingSince(2*testTimeout)),
			},
		},
		{
			name:    "pending pod of another revision",
			podName: testPod,
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old", inPhase(corev1.PodPending)),
			},
			expectedWait:  testTimeout,
			expectedEvent: "Warning " + ReasonPreviousRevisionStillRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				recorder := record.NewFakeRecorder(10)
				sink := &levelSink{}
				p := newPreviousPods(t, newFakeReader(t, tt.pods...), recorder, sink)
				p.Name = tt.podName

				start := time.Now()
				err := p.Wait(t.Context())

				require.NoError(t, err)
				assert.Equal(t, tt.expectedWait, time.Since(start))
				if tt.expectedEvent == "" {
					assert.Empty(t, recorder.Events)
					assert.Zero(t, sink.errorLines(), "no error-level log without giving up")
					return
				}
				require.Len(t, recorder.Events, 1)
				event := <-recorder.Events
				assert.Contains(t, event, tt.expectedEvent)
				assert.Contains(t, event, "multitenancy-manager-old-a")
				assert.Equal(t, 1, sink.errorLines(), "giving up is logged once at error level")
			})
		})
	}
}

// TestPreviousPods_Wait_PodGoesAway checks that Wait returns at the first check after the last pod of
// another revision is gone, and not at the timeout.
func TestPreviousPods_Wait_PodGoesAway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		old := newPod("multitenancy-manager-old-a", "old", terminating)
		reader := newFakeReader(t, newPod(testPod, "new"), old)
		recorder := record.NewFakeRecorder(10)
		sink := &levelSink{}
		p := newPreviousPods(t, reader, recorder, sink)

		var waitErr error
		done := make(chan struct{})
		go func() {
			defer close(done)
			waitErr = p.Wait(t.Context())
		}()

		// Two checks later the pod is still terminating, and Wait still waits.
		time.Sleep(2 * testInterval)
		synctest.Wait()
		select {
		case <-done:
			require.FailNow(t, "Wait returned while a pod of another revision was terminating")
		default:
		}

		// The kubelet finishes the pod: the finalizer goes, the object goes with it.
		finished := getPod(t, reader, old.Name)
		finished.Finalizers = nil
		require.NoError(t, reader.Update(t.Context(), finished))
		start := time.Now()

		<-done
		require.NoError(t, waitErr)
		assert.Equal(t, testInterval, time.Since(start), "Wait returns at the next check")
		assert.Empty(t, recorder.Events)
		assert.Contains(t, sink.infoLines(), "the pods of other revisions finished")
	})
}

// TestPreviousPods_Wait_LogsAStartWithoutWaiting checks that Wait leaves a line when it has nothing to
// wait for, so the log tells a start without waiting from a pod that is still waiting for the lease.
func TestPreviousPods_Wait_LogsAStartWithoutWaiting(t *testing.T) {
	tests := []struct {
		name string
		pods []client.Object
	}{
		{name: "no pod of another revision", pods: []client.Object{newPod(testPod, "new")}},
		{
			name: "a pod of another revision deleted longer ago than the timeout",
			pods: []client.Object{
				newPod(testPod, "new"),
				newPod("multitenancy-manager-old-a", "old", terminatingSince(2*testTimeout)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				sink := &levelSink{}
				p := newPreviousPods(t, newFakeReader(t, tt.pods...), record.NewFakeRecorder(10), sink)

				require.NoError(t, p.Wait(t.Context()))

				assert.Equal(t, []string{"no pods of other revisions to wait for"}, sink.infoLines())
			})
		})
	}
}

// TestPreviousPods_Wait_ListFails checks that a failed list is retried at the next check instead of
// ending the wait.
func TestPreviousPods_Wait_ListFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		failures := 1
		reader := newFakeReaderWith(t, interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if failures > 0 {
					failures--
					return errors.New("apiserver is unavailable")
				}
				return c.List(ctx, list, opts...)
			},
		}, newPod(testPod, "new"))
		recorder := record.NewFakeRecorder(10)
		p := newPreviousPods(t, reader, recorder, &levelSink{})

		start := time.Now()
		err := p.Wait(t.Context())

		require.NoError(t, err)
		assert.Equal(t, testInterval, time.Since(start), "the failed check is retried after one interval")
		assert.Empty(t, recorder.Events)
	})
}

// TestPreviousPods_Wait_Cancelled checks that a cancelled context ends the wait with its error and
// records no event.
func TestPreviousPods_Wait_Cancelled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader := newFakeReader(t, newPod(testPod, "new"), newPod("multitenancy-manager-old-a", "old"))
		recorder := record.NewFakeRecorder(10)
		p := newPreviousPods(t, reader, recorder, &levelSink{})
		ctx, cancel := context.WithTimeout(t.Context(), 2*testInterval)
		defer cancel()

		err := p.Wait(ctx)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Empty(t, recorder.Events)
	})
}

func newPreviousPods(t *testing.T, reader client.Reader, recorder record.EventRecorder, sink *levelSink) *PreviousPods {
	t.Helper()
	return &PreviousPods{
		Reader:    reader,
		Recorder:  recorder,
		Logger:    logr.New(sink),
		Namespace: testNamespace,
		Name:      testPod,
		Selector:  testSelector,
		Interval:  testInterval,
		Timeout:   testTimeout,
	}
}

func newFakeReader(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return newFakeReaderWith(t, interceptor.Funcs{}, objs...)
}

func newFakeReaderWith(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithInterceptorFuncs(funcs).Build()
}

func getPod(t *testing.T, c client.Reader, name string) *corev1.Pod {
	t.Helper()
	pod := new(corev1.Pod)
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, pod))
	return pod
}

// newPod returns a running pod of the controller Deployment whose revision is hash.
func newPod(name, hash string, mutators ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				"app":                                  "multitenancy-manager",
				appsv1.DefaultDeploymentUniqueLabelKey: hash,
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	for _, mutate := range mutators {
		mutate(pod)
	}
	return pod
}

func inPhase(phase corev1.PodPhase) func(*corev1.Pod) {
	return func(pod *corev1.Pod) { pod.Status.Phase = phase }
}

func inNamespace(namespace string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) { pod.Namespace = namespace }
}

func withLabel(key, value string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) { pod.Labels[key] = value }
}

func withoutLabel(key string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) { delete(pod.Labels, key) }
}

// replicaSet returns the ReplicaSet of the controller Deployment for the pods of revision hash.
func replicaSet(hash string, revision int) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Name:        "multitenancy-manager-" + hash,
		Namespace:   testNamespace,
		Labels:      map[string]string{"app": "multitenancy-manager", appsv1.DefaultDeploymentUniqueLabelKey: hash},
		Annotations: map[string]string{deploymentRevisionAnnotation: strconv.Itoa(revision)},
	}}
}

// synctestEpoch is the time a synctest bubble starts at; the test table is built outside of it.
var synctestEpoch = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

// terminatingSince marks the pod deleted the given time before the bubble starts, its containers
// still stopping.
func terminatingSince(ago time.Duration) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		deleted := metav1.NewTime(synctestEpoch.Add(-ago))
		pod.DeletionTimestamp = &deleted
		pod.Finalizers = []string{testFinalizer}
	}
}

// terminating marks the pod deleted while its containers still stop; the finalizer keeps the object
// the way the kubelet does until the pod is finished.
func terminating(pod *corev1.Pod) {
	now := metav1.Now()
	pod.DeletionTimestamp = &now
	pod.Finalizers = []string{testFinalizer}
}

// levelSink is a logr.LogSink that counts the lines logged at error level; logr has no warning level,
// so that is where a warning goes. It also keeps the messages of the info lines.
type levelSink struct {
	mu          sync.Mutex
	errorsCount int
	infos       []string
}

var _ logr.LogSink = (*levelSink)(nil)

func (*levelSink) Init(logr.RuntimeInfo) {}

func (*levelSink) Enabled(int) bool { return true }

func (s *levelSink) Info(_ int, msg string, _ ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infos = append(s.infos, msg)
}

func (s *levelSink) infoLines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.infos)
}

func (s *levelSink) Error(error, string, ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorsCount++
}

func (s *levelSink) WithValues(...any) logr.LogSink { return s }

func (s *levelSink) WithName(string) logr.LogSink { return s }

func (s *levelSink) errorLines() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errorsCount
}
