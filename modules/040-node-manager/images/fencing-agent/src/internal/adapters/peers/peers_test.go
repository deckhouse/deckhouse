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

package peers

import (
	"context"
	"net/netip"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const (
	self     = "node-a"
	imageNew = "registry.example/fencing-agent@sha256:2222"
	imageOld = "registry.example/fencing-agent@sha256:1111"
)

func agentPod(uid, node, hostIP, img string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fencing-agent-" + uid,
			Namespace: Namespace,
			UID:       types.UID(uid),
			Labels: map[string]string{
				"app": "fencing-agent",
				"node-manager.deckhouse.io/fencing-agent-version": "2",
			},
		},
		Spec: corev1.PodSpec{
			NodeName:   node,
			Containers: []corev1.Container{{Name: agentContainer, Image: img}},
		},
		Status: corev1.PodStatus{HostIP: hostIP},
	}
}

func names(peers []domain.Peer) []string {
	out := make([]string, 0, len(peers))
	for _, p := range peers {
		out = append(out, p.Name)
	}

	return out
}

func newList(objects ...runtime.Object) *List {
	return New(fake.NewClientset(objects...), self, log.NewNop())
}

func TestPodsGiveNodeNameAndHostIP(t *testing.T) {
	l := newList()

	l.onPod(agentPod("own", self, "10.0.0.1", imageNew))
	l.onPod(agentPod("b", "node-b", "10.0.0.2", imageNew))
	l.onPod(agentPod("c", "node-c", "10.0.0.3", imageOld))

	want := []domain.Peer{
		{Name: "node-b", Addr: netip.MustParseAddr("10.0.0.2"), Compatible: true},
		{Name: "node-c", Addr: netip.MustParseAddr("10.0.0.3"), Compatible: false},
	}
	if got := l.Peers(); !slices.Equal(got, want) {
		t.Errorf("Peers() = %v, want %v: the own node is not a peer, the image decides compatibility", got, want)
	}
}

func TestPendingPodWithoutAddressIsSkipped(t *testing.T) {
	l := newList()

	l.onPod(agentPod("b", "node-b", "", imageNew))

	if got := l.Peers(); len(got) != 0 {
		t.Errorf("Peers() = %v, want none: a Pending pod without hostIP is not a peer", got)
	}
}

func TestOnlyIPv4HostIPIsAnAddress(t *testing.T) {
	l := newList()

	pod := agentPod("b", "node-b", "fd00::2", imageNew)
	pod.Status.HostIPs = []corev1.HostIP{{IP: "fd00::2"}, {IP: "10.0.0.2"}}
	l.onPod(pod)

	got := l.Peers()
	if len(got) != 1 || got[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Errorf("Peers() = %v, want node-b at the IPv4 host address 10.0.0.2", got)
	}
}

// A peer pod in CrashLoopBackOff: the pod stays, so the node stays.
func TestCrashLoopAndTerminatingPodsStay(t *testing.T) {
	l := newList()

	crash := agentPod("b", "node-b", "10.0.0.2", imageNew)
	crash.Status.Phase = corev1.PodRunning
	crash.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  agentContainer,
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
	}}
	l.onPod(crash)

	terminating := agentPod("c", "node-c", "10.0.0.3", imageNew)
	now := metav1.Now()
	terminating.DeletionTimestamp = &now
	l.onPod(terminating)

	if got := names(l.Peers()); !slices.Equal(got, []string{"node-b", "node-c"}) {
		t.Errorf("Peers() = %v, want [node-b node-c]", got)
	}
}

// A peer pod disappeared without 'leaving'.
func TestNodeIsRemovedSixtySecondsAfterItsPodIsGone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList()
		pod := agentPod("b", "node-b", "10.0.0.2", imageNew)

		l.onPod(pod)
		l.onPodDelete(pod)

		time.Sleep(removeDelay - time.Millisecond)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Fatalf("Peers() = %v just before 60 s, want [node-b]", got)
		}

		time.Sleep(time.Millisecond)
		if got := l.Peers(); len(got) != 0 {
			t.Errorf("Peers() = %v at 60 s after the pod is gone, want none", got)
		}
	})
}

func TestNewPodBeforeSixtySecondsKeepsTheNode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList()

		old := agentPod("b1", "node-b", "10.0.0.2", imageOld)
		l.onPod(old)
		l.onPodDelete(cache.DeletedFinalStateUnknown{Key: Namespace + "/" + old.Name, Obj: old})

		time.Sleep(30 * time.Second)
		l.onPod(agentPod("b2", "node-b", "10.0.0.2", imageNew))

		time.Sleep(time.Hour)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Errorf("Peers() = %v, want [node-b]: the new pod replaced the old one", got)
		}
	})
}

func TestDeleteOfOnePodOfTwoKeepsTheNode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList()

		old := agentPod("b1", "node-b", "10.0.0.2", imageOld)
		l.onPod(old)
		l.onPod(agentPod("b2", "node-b", "10.0.0.2", imageNew))
		l.onPodDelete(old)

		time.Sleep(time.Hour)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Errorf("Peers() = %v, want [node-b]: the node still has a pod", got)
		}
	})
}

// A peer left gracefully: the list keeps "leaving" for 10 s, then the usual
// rules apply.
func TestLeavingNodeIsOutForTenSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList()
		pod := agentPod("b", "node-b", "10.0.0.2", imageNew)
		l.onPod(pod)

		l.Remove("node-b")
		if got := l.Peers(); len(got) != 0 {
			t.Fatalf("Peers() = %v, want none after leaving", got)
		}
		if !l.Leaving("node-b") {
			t.Fatal("Leaving(node-b) = false, want true in the hold")
		}
		// The hold keeps the node out of the polls, but the agent answers it.
		if got, ok := l.Peer("node-b"); !ok || got.Addr != netip.MustParseAddr("10.0.0.2") {
			t.Fatalf("Peer(node-b) = %v, %v, want the node in the hold", got, ok)
		}

		// The terminating pod keeps changing, memberlist still gossips the
		// node, the copies of "leaving" come.
		now := metav1.Now()
		pod.DeletionTimestamp = &now
		l.onPod(pod)
		l.AddMember("node-b", netip.MustParseAddr("10.0.0.2"))
		time.Sleep(leavingHold - time.Second)
		l.Remove("node-b")
		if got := l.Peers(); len(got) != 0 {
			t.Fatalf("Peers() = %v, want none: events in 10 s do not add the node again", got)
		}

		// The pod is gone: after the hold the node stays for 60 s from the loss of the pod.
		l.onPodDelete(pod)
		gone := time.Now()
		time.Sleep(time.Second)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Fatalf("Peers() = %v, want [node-b] after the hold", got)
		}
		if l.Leaving("node-b") {
			t.Fatal("Leaving(node-b) = true, want false after the hold")
		}

		time.Sleep(removeDelay - time.Since(gone))
		if got, ok := l.Peer("node-b"); ok {
			t.Errorf("Peer(node-b) = %v, want none 60 s after the loss of the pod", got)
		}
		if got := l.Peers(); len(got) != 0 {
			t.Errorf("Peers() = %v, want none 60 s after the loss of the pod", got)
		}
	})
}

// kubelet restarts the agent container in the same pod after SIGTERM (a
// liveness failure): the pod UID stays, the node sent "leaving".
func TestContainerRestartInTheSamePodReturnsTheNode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pod := agentPod("b", "node-b", "10.0.0.2", imageNew)
		l := newList(pod)
		l.onPod(pod)
		l.Remove("node-b")

		restarted := pod.DeepCopy()
		restarted.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: agentContainer, RestartCount: 1, Ready: true}}
		l.onPod(restarted)
		l.AddMember("node-b", netip.MustParseAddr("10.0.0.2"))

		time.Sleep(leavingHold)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Fatalf("Peers() = %v, want [node-b] 10 s after leaving", got)
		}

		// "leaving" after the restart works again.
		l.Remove("node-b")
		if got := l.Peers(); len(got) != 0 {
			t.Fatalf("Peers() = %v, want none after the second leaving", got)
		}
	})
}

// A new memberlist member.
func TestMemberlistMemberIsAdded(t *testing.T) {
	l := newList()

	l.AddMember("node-b", netip.MustParseAddr("10.0.0.2"))
	l.AddMember(self, netip.MustParseAddr("10.0.0.1"))

	want := []domain.Peer{{Name: "node-b", Addr: netip.MustParseAddr("10.0.0.2")}}
	if got := l.Peers(); !slices.Equal(got, want) {
		t.Errorf("Peers() = %v, want %v", got, want)
	}
}

func TestMemberlistMemberStaysWithoutTheAPI(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList()

		l.AddMember("node-b", netip.MustParseAddr("10.0.0.2"))

		time.Sleep(time.Hour)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Errorf("Peers() = %v, want [node-b]: only a LIST without its pod starts the removal", got)
		}
	})
}

func TestMemberlistDoesNotChangeAPodAddress(t *testing.T) {
	l := newList()

	l.onPod(agentPod("b", "node-b", "10.0.0.2", imageNew))
	l.AddMember("node-b", netip.MustParseAddr("192.0.2.2"))

	if got := l.Peers(); got[0].Addr != netip.MustParseAddr("10.0.0.2") {
		t.Errorf("address = %s, want the hostIP 10.0.0.2", got[0].Addr)
	}
}

func TestRelistAddsMissedPodsAndStartsRemovalOfGonePods(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList(
			agentPod("own", self, "10.0.0.1", imageNew),
			agentPod("c", "node-c", "10.0.0.3", imageNew),
		)

		// node-b lost its delete event, node-c lost its add event, node-d is only in memberlist.
		l.onPod(agentPod("b", "node-b", "10.0.0.2", imageNew))
		l.AddMember("node-d", netip.MustParseAddr("10.0.0.4"))

		l.relist(t.Context())

		want := []domain.Peer{
			{Name: "node-b", Addr: netip.MustParseAddr("10.0.0.2")},
			{Name: "node-c", Addr: netip.MustParseAddr("10.0.0.3"), Compatible: true},
			{Name: "node-d", Addr: netip.MustParseAddr("10.0.0.4")},
		}
		if got := l.Peers(); !slices.Equal(got, want) {
			t.Fatalf("Peers() after LIST = %v, want %v", got, want)
		}

		time.Sleep(removeDelay)
		if got := names(l.Peers()); !slices.Equal(got, []string{"node-c"}) {
			t.Errorf("Peers() 60 s after LIST = %v, want [node-c]", got)
		}
	})
}

func TestRelistKeepsALeavingNodeOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newList(agentPod("b", "node-b", "10.0.0.2", imageNew))
		l.onPod(agentPod("b", "node-b", "10.0.0.2", imageNew))
		l.Remove("node-b")

		l.relist(t.Context())

		if got := l.Peers(); len(got) != 0 {
			t.Errorf("Peers() = %v, want none: the LIST does not undo leaving", got)
		}

		time.Sleep(leavingHold)
		l.relist(t.Context())

		if got := names(l.Peers()); !slices.Equal(got, []string{"node-b"}) {
			t.Errorf("Peers() = %v, want [node-b] after the hold", got)
		}
	})
}

func countLists(client *fake.Clientset) *atomic.Int32 {
	var lists atomic.Int32
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		lists.Add(1)
		return false, nil, nil
	})

	return &lists
}

// The safety LIST is off.
func TestRefreshIntervalZeroTurnsTheSafetyListOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := fake.NewClientset()
		lists := countLists(client)
		l := New(client, self, log.NewNop())

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { l.refreshLoop(ctx); close(done) }()

		time.Sleep(24 * time.Hour)
		if got := lists.Load(); got != 0 {
			t.Errorf("LIST calls with interval 0 = %d, want 0", got)
		}

		l.SetRefreshInterval(5 * time.Minute)
		time.Sleep(10*time.Minute + time.Second)
		if got := lists.Load(); got != 2 {
			t.Errorf("LIST calls in 10 min with interval 5m = %d, want 2", got)
		}

		l.SetRefreshInterval(0)
		time.Sleep(time.Hour)
		if got := lists.Load(); got != 2 {
			t.Errorf("LIST calls after the interval is set to 0 = %d, want 2", got)
		}

		cancel()
		<-done
	})
}

func TestRunFollowsTheInformer(t *testing.T) {
	client := fake.NewClientset(
		agentPod("b", "node-b", "10.0.0.2", imageNew),
		// An agent 1.0 pod has no version label and is not a peer.
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: Namespace, Labels: map[string]string{"app": "fencing-agent"}},
			Spec:       corev1.PodSpec{NodeName: "node-x"},
			Status:     corev1.PodStatus{HostIP: "10.0.0.9"},
		},
	)
	l := New(client, self, log.NewNop())

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()

	waitFor(t, func() bool { return slices.Equal(names(l.Peers()), []string{"node-b"}) })

	if _, err := client.CoreV1().Pods(Namespace).Create(ctx, agentPod("c", "node-c", "10.0.0.3", imageNew), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create pod: %v", err)
	}
	waitFor(t, func() bool { return slices.Equal(names(l.Peers()), []string{"node-b", "node-c"}) })

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run() = %v, want nil", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached in 5 s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
