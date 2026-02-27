// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"deckhouse.io/webhook/internal/templater"
)

const (
	hooksBaseDir         = "hooks"
	webhooksSubDir       = "webhooks"
	conversionSubDir     = "conversion"
	webhookFileExtension = ".py"
	directoryPermissions = 0755
	filePermissions      = 0755
)

// ConversionWebhookReconciler reconciles a ConversionWebhook object
type ConversionWebhookReconciler struct {
	isReloadShellNeed *atomic.Bool
	client            client.Client
	scheme            *runtime.Scheme
	logger            *log.Logger
	pythonTemplate    string
}

// NewConversionWebhookReconciler creates a new ConversionWebhookReconciler.
// The isReloadShellNeed flag is shared between reconcilers to signal shell-operator reload.
func NewConversionWebhookReconciler(
	k8sClient client.Client,
	scheme *runtime.Scheme,
	logger *log.Logger,
	pythonTemplate string,
	isReloadShellNeed *atomic.Bool,
) *ConversionWebhookReconciler {
	return &ConversionWebhookReconciler{
		isReloadShellNeed: isReloadShellNeed,
		client:            k8sClient,
		scheme:            scheme,
		logger:            logger.Named("conversion-webhook"),
		pythonTemplate:    pythonTemplate,
	}
}

// webhookDir returns the directory path for a webhook's files.
// Example: hooks/deckhouse/webhooks/conversion
func (r *ConversionWebhookReconciler) webhookDir(webhookName string) string {
	return filepath.Join(hooksBaseDir, webhookName, webhooksSubDir, conversionSubDir)
}

// webhookFilePath returns the full path to a webhook's Python file.
// Example: hooks/deckhouse/webhooks/conversion/deckhouse.py
func (r *ConversionWebhookReconciler) webhookFilePath(webhookName string) string {
	return filepath.Join(r.webhookDir(webhookName), webhookName+webhookFileExtension)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ConversionWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var res ctrl.Result

	r.logger.Debug("conversion webhook processing started", slog.String("resource_name", req.Name))
	defer func() {
		r.logger.Debug("conversion webhook processing complete", slog.String("resource_name", req.Name), slog.Any("reconcile_result", res))
	}()

	webhook := new(deckhouseiov1alpha1.ConversionWebhook)
	err := r.client.Get(ctx, req.NamespacedName, webhook)
	if err != nil {
		r.logger.Warn("error get resource", slog.String("name", req.Name))
		// resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return res, nil
		}

		r.logger.Debug("get conversion webhook", log.Err(err))

		return res, err
	}

	// resource marked as "to delete"
	r.logger.Debug("debug deletion timestamp", slog.Any("timestamp", webhook.DeletionTimestamp))
	if !webhook.DeletionTimestamp.IsZero() {
		r.logger.Debug("conversion webhook deletion", slog.String("deletion_timestamp", webhook.DeletionTimestamp.String()))

		res, err := r.handleDeleteConversionWebhook(ctx, webhook)
		if err != nil {
			r.logger.Warn("delete conversion webhook", log.Err(err))

			return res, err
		}
		return res, nil
	}

	res, err = r.handleProcessConversionWebhook(ctx, webhook)
	if err != nil {
		r.logger.Warn("process conversion webhook", log.Err(err))

		return res, err
	}

	return res, nil
}

func (r *ConversionWebhookReconciler) handleProcessConversionWebhook(ctx context.Context, cwh *deckhouseiov1alpha1.ConversionWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	webhookDir := r.webhookDir(cwh.Name)
	if err := os.MkdirAll(webhookDir, directoryPermissions); err != nil {
		r.logger.Error("failed to create directory", slog.String("path", webhookDir), log.Err(err))
		return res, fmt.Errorf("create dir %s: %w", webhookDir, err)
	}

	buf, err := templater.RenderConversionTemplate(r.pythonTemplate, cwh)
	if err != nil {
		return res, fmt.Errorf("render template: %w", err)
	}

	webhookFile := r.webhookFilePath(cwh.Name)
	if err := os.WriteFile(webhookFile, buf.Bytes(), filePermissions); err != nil {
		r.logger.Error("failed to write webhook file", slog.String("path", webhookFile), log.Err(err))
		return res, fmt.Errorf("write file %s: %w", webhookFile, err)
	}

	r.isReloadShellNeed.Store(true)

	// add finalizers
	needsUpdate := false
	if !controllerutil.ContainsFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookCRDCleanupFinalizer) {
		controllerutil.AddFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookCRDCleanupFinalizer)
		needsUpdate = true
	}
	if !controllerutil.ContainsFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer) {
		controllerutil.AddFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer)
		needsUpdate = true
	}

	if needsUpdate {
		r.logger.Debug("add finalizers")
		if err := r.client.Update(ctx, cwh); err != nil {
			if removeErr := os.Remove(webhookFile); removeErr != nil {
				r.logger.Warn("failed to cleanup webhook file", log.Err(removeErr))
			}
			return res, fmt.Errorf("add finalizers: %w", err)
		}
	}

	return res, nil
}

func (r *ConversionWebhookReconciler) handleDeleteConversionWebhook(ctx context.Context, cwh *deckhouseiov1alpha1.ConversionWebhook) (ctrl.Result, error) {
	var res ctrl.Result

	// Clean up the target CRD's conversion config before removing FS files.
	// So it won't try to route conversion requests to a non-existent webhook.
	if controllerutil.ContainsFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookCRDCleanupFinalizer) {
		if err := r.cleanupCRDConversion(ctx, cwh.Name); err != nil {
			return res, fmt.Errorf("cleanup CRD conversion for %s: %w", cwh.Name, err)
		}

		r.logger.Debug("remove crd-cleanup finalizer")
		controllerutil.RemoveFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookCRDCleanupFinalizer)
		if err := r.client.Update(ctx, cwh); err != nil {
			return res, fmt.Errorf("remove crd-cleanup finalizer for %s: %w", cwh.Name, err)
		}

		// Return so the next reconcile handles FS cleanup
		return res, nil
	}

	// Clean up the filesystem.
	webhookFile := r.webhookFilePath(cwh.Name)
	if err := os.Remove(webhookFile); err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("delete webhook file %s: %w", webhookFile, err)
	}

	r.isReloadShellNeed.Store(true)

	// remove exist-on-fs finalizer.
	if controllerutil.ContainsFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer) {
		r.logger.Debug("remove exist-on-fs finalizer")
		controllerutil.RemoveFinalizer(cwh, deckhouseiov1alpha1.ConversionWebhookFinalizer)

		if err := r.client.Update(ctx, cwh); err != nil {
			return res, fmt.Errorf("remove finalizer for %s: %w", cwh.Name, err)
		}
	}

	return res, nil
}

// cleanupCRDConversion resets the target CRD's conversion strategy to None,
// removing the webhook configuration that shell-operator had injected.
func (r *ConversionWebhookReconciler) cleanupCRDConversion(ctx context.Context, crdName string) error {
	crd := new(apiextensionsv1.CustomResourceDefinition)
	err := r.client.Get(ctx, types.NamespacedName{Name: crdName}, crd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("target CRD not found, skipping conversion cleanup", slog.String("crd_name", crdName))
			return nil
		}
		return fmt.Errorf("get CRD %s: %w", crdName, err)
	}

	if crd.Spec.Conversion == nil || crd.Spec.Conversion.Strategy != apiextensionsv1.WebhookConverter {
		r.logger.Debug("CRD has no webhook conversion, skipping cleanup", slog.String("crd_name", crdName))
		return nil
	}

	r.logger.Info("resetting CRD conversion strategy to None", slog.String("crd_name", crdName))
	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.NoneConverter,
	}

	if err := r.client.Update(ctx, crd); err != nil {
		return fmt.Errorf("update CRD %s: %w", crdName, err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConversionWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhouseiov1alpha1.ConversionWebhook{}).
		Named("conversionwebhook").
		Complete(r)
}
