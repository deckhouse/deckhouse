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
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueWaitPod          = 5 * time.Second
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client   client.Client
	log      *log.Logger
	node     NodeIdentity
	commands map[controlplanev1alpha1.CommandName]Command
}

func Register(mgr manager.Manager) error {
	node, err := nodeIdentityFromEnv()
	if err != nil {
		return fmt.Errorf("read node identity: %w", err)
	}

	r := &Reconciler{
		client:   mgr.GetClient(),
		log:      log.Default().With(slog.String("controller", constants.CpoControllerName)),
		node:     node,
		commands: defaultCommands(),
	}
	// Inject Reconciler-level deps into commands that need them.
	r.commands[controlplanev1alpha1.CommandWaitPodReady].(*waitPodReadyCommand).waitForPod = r.waitForPod

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: node.Name,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	cpoPredicate := predicate.And(nodeLabelPredicate, approvedCPOPredicate())
	podPredicate := controlPlanePodPredicate(node.Name)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(false),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.CpoControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(cpoPredicate),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodToOperations),
			builder.WithPredicates(podPredicate),
		).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	logger := r.log.With(slog.String("operation", req.Name))

	op := &controlplanev1alpha1.ControlPlaneOperation{}
	if err := r.client.Get(ctx, req.NamespacedName, op); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !op.Spec.Approved || op.IsTerminal() {
		return reconcile.Result{}, nil
	}

	state := controlplanev1alpha1.NewOperationState(op)

	logger.Info("reconciling operation",
		slog.String("component", string(op.Spec.Component)),
		slog.Any("commands", op.Spec.Commands))
	defer func() {
		if err != nil {
			logger.Error("reconcile failed", log.Err(err))
		} else {
			logger.Info("reconcile finished")
		}
	}()

	// CertObserver is read-only, no secrets needed
	if op.Spec.Component == controlplanev1alpha1.OperationComponentCertObserver {
		return r.reconcilePipeline(ctx, state, nil, nil, logger)
	}

	cpmSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name:      constants.ControlPlaneManagerConfigSecretName,
		Namespace: constants.KubeSystemNamespace,
	}, cpmSecret); err != nil {
		return reconcile.Result{}, fmt.Errorf("get cpm secret: %w", err)
	}

	pkiSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name:      constants.PkiSecretName,
		Namespace: constants.KubeSystemNamespace,
	}, pkiSecret); err != nil {
		return reconcile.Result{}, fmt.Errorf("get pki secret: %w", err)
	}

	// Renewal operation not needed isDesiredStale check
	if op.IsRenewalOperation() {
		return r.reconcilePipeline(ctx, state, cpmSecret.Data, pkiSecret.Data, logger)
	}

	// Verify that the secret content matches what this operation was created for.
	if stale, reason := r.isDesiredStale(op, cpmSecret.Data, pkiSecret.Data); stale {
		if recoveredCmd, recovered, recoverErr := r.recoverInProgressCommitPoint(ctx, state); recoverErr != nil {
			return reconcile.Result{}, recoverErr
		} else if recovered {
			logger.Info("recovered in-progress commit-point from disk state", slog.String("command", string(recoveredCmd)))
		}

		logger.Info("desired checksums stale, cancelling", slog.String("reason", reason))
		state.SetReadyReason(constants.ReasonCancelled, reason)
		return reconcile.Result{}, r.patchStatus(ctx, state)
	}

	return r.reconcilePipeline(ctx, state, cpmSecret.Data, pkiSecret.Data, logger)
}

// isDesiredStale checks that secret content still matches with desired checksums in the operation spec.
// Returns true with reason string if stale.
func (r *Reconciler) isDesiredStale(op *controlplanev1alpha1.ControlPlaneOperation, cpmSecretData, pkiSecretData map[string][]byte) (bool, string) {
	component := op.Spec.Component

	if component == controlplanev1alpha1.OperationComponentHotReload {
		freshConfig, err := checksum.HotReloadChecksum(cpmSecretData)
		if err != nil {
			return true, fmt.Sprintf("failed to calculate hot-reload checksum: %v", err)
		}
		if op.Spec.DesiredConfigChecksum != freshConfig {
			return true, fmt.Sprintf("hot-reload config checksum changed: desired %s, current %s",
				op.Spec.DesiredConfigChecksum, freshConfig)
		}
		return false, ""
	}

	podName := component.PodComponentName()

	freshConfig, err := checksum.ComponentChecksum(cpmSecretData, podName)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate config checksum: %v", err)
	}
	if op.Spec.DesiredConfigChecksum != "" && op.Spec.DesiredConfigChecksum != freshConfig {
		return true, fmt.Sprintf("config checksum changed: desired %s, current %s",
			op.Spec.DesiredConfigChecksum, freshConfig)
	}

	freshPKI, err := checksum.ComponentPKIChecksum(cpmSecretData, podName)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate pki checksum: %v", err)
	}
	if op.Spec.DesiredPKIChecksum != freshPKI {
		return true, fmt.Sprintf("pki checksum changed: desired %s, current %s",
			op.Spec.DesiredPKIChecksum, freshPKI)
	}

	freshCA, err := checksum.PKIChecksum(pkiSecretData)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate ca checksum: %v", err)
	}
	if op.Spec.DesiredCAChecksum != "" && op.Spec.DesiredCAChecksum != freshCA {
		return true, fmt.Sprintf("ca checksum changed: desired %s, current %s",
			op.Spec.DesiredCAChecksum, freshCA)
	}

	return false, ""
}

func (r *Reconciler) recoverInProgressCommitPoint(ctx context.Context, state *controlplanev1alpha1.OperationState) (controlplanev1alpha1.CommandName, bool, error) {
	op := state.Raw()
	cmd, ok := inProgressCommitPoint(op)
	if !ok {
		return "", false, nil
	}

	matches, err := r.diskMatchesDesired(op, cmd)
	if err != nil {
		return "", false, fmt.Errorf("check disk state for %s: %w", cmd, err)
	}
	if !matches {
		return cmd, false, nil
	}

	state.MarkCommandCompleted(cmd)
	if err := r.patchStatus(ctx, state); err != nil {
		return "", false, fmt.Errorf("persist recovered command %s: %w", cmd, err)
	}
	return cmd, true, nil
}

func inProgressCommitPoint(op *controlplanev1alpha1.ControlPlaneOperation) (controlplanev1alpha1.CommandName, bool) {
	switch {
	case op.IsCommandInProgress(controlplanev1alpha1.CommandSyncManifests):
		return controlplanev1alpha1.CommandSyncManifests, true
	case op.IsCommandInProgress(controlplanev1alpha1.CommandJoinEtcdCluster):
		return controlplanev1alpha1.CommandJoinEtcdCluster, true
	case op.IsCommandInProgress(controlplanev1alpha1.CommandSyncHotReload):
		return controlplanev1alpha1.CommandSyncHotReload, true
	default:
		return "", false
	}
}

func (r *Reconciler) diskMatchesDesired(op *controlplanev1alpha1.ControlPlaneOperation, cmd controlplanev1alpha1.CommandName) (bool, error) {
	switch cmd {
	case controlplanev1alpha1.CommandSyncManifests:
		return manifestMatchesDesired(op)
	case controlplanev1alpha1.CommandJoinEtcdCluster:
		manifestMatches, err := manifestMatchesDesired(op)
		if err != nil || !manifestMatches {
			return manifestMatches, err
		}
		peerURL := etcd.GetPeerURL(r.node.AdvertiseIP)
		memberExists, err := checkEtcdMemberExists(r.node.Name, peerURL, constants.KubernetesPkiPath, r.node.KubeconfigDir)
		if err != nil {
			return false, err
		}
		return memberExists, nil
	case controlplanev1alpha1.CommandSyncHotReload:
		diskChecksum, err := hotReloadChecksumFromDisk(constants.ExtraFilesPath)
		if err != nil {
			return false, err
		}
		return diskChecksum == op.Spec.DesiredConfigChecksum, nil
	default:
		return false, nil
	}
}

func manifestMatchesDesired(op *controlplanev1alpha1.ControlPlaneOperation) (bool, error) {
	podComponent := op.Spec.Component.PodComponentName()
	if podComponent == "" {
		return false, nil
	}

	path := filepath.Join(constants.ManifestsPath, podComponent+".yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read manifest %s: %w", path, err)
	}

	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(content, pod); err != nil {
		return false, fmt.Errorf("unmarshal manifest %s: %w", path, err)
	}

	annotations := pod.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	if op.Spec.DesiredConfigChecksum != "" && annotations[constants.ConfigChecksumAnnotationKey] != op.Spec.DesiredConfigChecksum {
		return false, nil
	}
	if op.Spec.DesiredPKIChecksum != "" && annotations[constants.PKIChecksumAnnotationKey] != op.Spec.DesiredPKIChecksum {
		return false, nil
	}
	if op.Spec.DesiredCAChecksum != "" && annotations[constants.CAChecksumAnnotationKey] != op.Spec.DesiredCAChecksum {
		return false, nil
	}
	if op.IsRenewalOperation() && annotations[constants.CertRenewalIDAnnotationKey] != op.CertRenewalID() {
		return false, nil
	}

	return true, nil
}

func hotReloadChecksumFromDisk(extraFilesDir string) (string, error) {
	hash := sha256.New()
	for _, key := range checksum.HotReloadChecksumDependsOn {
		filePath := filepath.Join(extraFilesDir, strings.TrimPrefix(key, "extra-file-"))
		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("read hot-reload file %s: %w", filePath, err)
		}
		if _, err := hash.Write(content); err != nil {
			return "", fmt.Errorf("hash hot-reload file %s: %w", filePath, err)
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
