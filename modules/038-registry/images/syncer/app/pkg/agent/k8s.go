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

package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// LeaderRoleLabel is set on the holder's pod; the registry-cache-leader Service
// selects it to route writes (and the followers' mirror source).
const LeaderRoleLabel = "registry-cache-role"

// LeaderManager runs leader election and reflects leadership into the pod label
// + the shared isLeader flag.
type LeaderManager struct {
	client    kubernetes.Interface
	logger    *slog.Logger
	namespace string
	podName   string
	leaseName string
	isLeader  *atomic.Bool
}

func NewLeaderManager(client kubernetes.Interface, logger *slog.Logger, namespace, podName, leaseName string, isLeader *atomic.Bool) *LeaderManager {
	return &LeaderManager{client: client, logger: logger, namespace: namespace, podName: podName, leaseName: leaseName, isLeader: isLeader}
}

// Run blocks running leader election until ctx is cancelled.
func (m *LeaderManager) Run(ctx context.Context) {
	lock := &resourcelock.LeaseLock{
		LeaseMeta:  metaObjectMeta(m.namespace, m.leaseName),
		Client:     m.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{Identity: fmt.Sprintf("%s_%s", m.podName, uuid.NewString())},
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				m.isLeader.Store(true)
				if err := m.setLeaderLabel(ctx, true); err != nil {
					m.logger.Error("set leader label", "error", err)
				}
				m.logger.Info("became leader")
			},
			OnStoppedLeading: func() {
				m.isLeader.Store(false)
				// Best-effort: drop the label so the leader Service repoints.
				if err := m.setLeaderLabel(context.Background(), false); err != nil {
					m.logger.Error("clear leader label", "error", err)
				}
				m.logger.Info("lost leadership")
			},
			OnNewLeader: func(id string) { m.logger.Info("new leader observed", "id", id) },
		},
	})
}

// setLeaderLabel sets or clears registry-cache-role=leader on this pod.
func (m *LeaderManager) setLeaderLabel(ctx context.Context, leader bool) error {
	var value any
	if leader {
		value = "leader"
	} else {
		value = nil // merge-patch null removes the key
	}
	patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:%s}}}`, LeaderRoleLabel, jsonValue(value)))
	_, err := m.client.CoreV1().Pods(m.namespace).Patch(ctx, m.podName, types.MergePatchType, patch, metaPatchOptions())
	return err
}

func metaObjectMeta(ns, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: ns, Name: name}
}
func metaPatchOptions() metav1.PatchOptions { return metav1.PatchOptions{} }
func jsonValue(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%q", v)
}
