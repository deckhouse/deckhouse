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

package controlplaneoperation

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	etcdclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

const (
	etcdDefragFragRatioThreshold = 0.20

	etcdDefragTimeout       = 2 * time.Minute
	etcdDefragStatusTimeout = 10 * time.Second
)

// defragEtcdIfNeeded runs defragmentation if the fragmented ratio exceeds etcdDefragFragRatioThreshold.
// Returns true if defragmentation was performed, false if it was skipped.
func defragEtcdIfNeeded(ctx context.Context, pkiDir, kubeconfigDir string, logger *log.Logger) (bool, error) {
	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")
	kubeClient, err := etcdclient.ClientSetFromFile(adminConfPath)
	if err != nil {
		return false, fmt.Errorf("create k8s client from admin.conf: %w", err)
	}

	etcdCli, err := etcdclient.New(kubeClient, pkiDir)
	if err != nil {
		return false, fmt.Errorf("create etcd client: %w", err)
	}
	defer etcdCli.Close()

	// Always connect to the local member directly — the controller runs on the same node.
	localEndpoint := etcd.GetClientURL("127.0.0.1")

	if err := etcdCli.CheckClusterHealthy(ctx, etcdDefragStatusTimeout); err != nil {
		return false, fmt.Errorf("etcd cluster not healthy, skipping defrag: %w", err)
	}

	statusCtx, cancel := context.WithTimeout(ctx, etcdDefragStatusTimeout)
	defer cancel()

	resp, err := etcdCli.Status(statusCtx, localEndpoint)
	if err != nil {
		return false, fmt.Errorf("get etcd status: %w", err)
	}

	if resp.DbSize <= 0 {
		logger.Info("etcd defrag skipped: db size is zero")
		return false, nil
	}

	fragmented := resp.DbSize - resp.DbSizeInUse
	fragRatio := float64(fragmented) / float64(resp.DbSize)

	logger.Info("etcd defrag: fragmentation check",
		slog.Int64("db_size_bytes", resp.DbSize),
		slog.Int64("db_size_in_use_bytes", resp.DbSizeInUse),
		slog.Int64("fragmented_bytes", fragmented),
		slog.Float64("frag_ratio", fragRatio),
		slog.Float64("threshold", etcdDefragFragRatioThreshold))

	if fragRatio < etcdDefragFragRatioThreshold {
		logger.Info("etcd defrag skipped: fragmentation below threshold")
		return false, nil
	}

	logger.Info("etcd defrag: starting")

	defragCtx, defragCancel := context.WithTimeout(ctx, etcdDefragTimeout)
	defer defragCancel()

	if err = etcdCli.Defragment(defragCtx, localEndpoint); err != nil {
		return false, fmt.Errorf("defragment etcd: %w", err)
	}

	logger.Info("etcd defrag: completed successfully")
	return true, nil
}

// defragEtcd is the Reconciler-level implementation of the DefragEtcd step.
// It ensures the etcd pod is Ready, then defragments if fragmentation exceeds the threshold.
func (r *Reconciler) defragEtcd(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (StepResult, error) {
	if state.Raw().Spec.Component != controlplanev1alpha1.OperationComponentEtcd {
		return StepResult{Outcome: OutcomeCompleted}, nil
	}

	podName := fmt.Sprintf("%s-%s",
		controlplanev1alpha1.OperationComponentEtcd.PodComponentName(),
		r.node.Name)
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name:      podName,
		Namespace: constants.KubeSystemNamespace,
	}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("etcd pod not found, will retry before defragmentation", slog.String("pod", podName))
			return StepResult{
				Outcome:      OutcomePending,
				Message:      "waiting for etcd pod to be ready before defragmentation",
				RequeueAfter: requeueWaitPod,
			}, nil
		}
		return StepResult{}, fmt.Errorf("get pod %s: %w", podName, err)
	}

	if !isPodReady(pod) {
		logger.Info("etcd pod not ready, will retry before defragmentation", slog.String("pod", podName))
		return StepResult{
			Outcome:      OutcomePending,
			Message:      "waiting for etcd pod to be ready before defragmentation",
			RequeueAfter: requeueWaitPod,
		}, nil
	}

	defragged, err := defragEtcdIfNeeded(ctx, constants.KubernetesPkiPath, r.node.KubeconfigDir, logger)
	if err != nil {
		return StepResult{}, err
	}

	if defragged {
		return StepResult{Outcome: OutcomeCompleted, Message: "defragmented"}, nil
	}
	return StepResult{Outcome: OutcomeCompleted, Message: "skipped: fragmentation below threshold"}, nil
}
