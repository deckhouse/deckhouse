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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/pkg/log"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"deckhouse.io/webhook/internal/templater"
)

// PresyncWebhookFiles lists all ConversionWebhook and ValidationWebhook CRs from
// the cluster and writes their rendered hook files to disk. This must be called
// before shell-operator starts so that it discovers all hooks on the first scan
// and does not require a reload triggered by controller reconciliation.
//
// The function intentionally does NOT set isReloadShellNeed because
// shell-operator has not started yet — there is nothing to reload.
func PresyncWebhookFiles(ctx context.Context, k8sClient client.Reader, conversionTpl, validationTpl string, logger *log.Logger) error {
	if err := presyncConversionWebhooks(ctx, k8sClient, conversionTpl, logger); err != nil {
		return fmt.Errorf("presync conversion webhooks: %w", err)
	}

	if err := presyncValidationWebhooks(ctx, k8sClient, validationTpl, logger); err != nil {
		return fmt.Errorf("presync validation webhooks: %w", err)
	}

	return nil
}

func presyncConversionWebhooks(ctx context.Context, k8sClient client.Reader, tpl string, logger *log.Logger) error {
	var list deckhouseiov1alpha1.ConversionWebhookList
	if err := k8sClient.List(ctx, &list); err != nil {
		return fmt.Errorf("list ConversionWebhook: %w", err)
	}

	for i := range list.Items {
		cwh := &list.Items[i]

		// Skip resources that are being deleted.
		if !cwh.DeletionTimestamp.IsZero() {
			continue
		}

		if err := writeConversionWebhookFile(cwh, tpl, logger); err != nil {
			return fmt.Errorf("write conversion webhook %s: %w", cwh.Name, err)
		}
	}

	return nil
}

func presyncValidationWebhooks(ctx context.Context, k8sClient client.Reader, tpl string, logger *log.Logger) error {
	var list deckhouseiov1alpha1.ValidationWebhookList
	if err := k8sClient.List(ctx, &list); err != nil {
		return fmt.Errorf("list ValidationWebhook: %w", err)
	}

	for i := range list.Items {
		vwh := &list.Items[i]

		// Skip resources that are being deleted.
		if !vwh.DeletionTimestamp.IsZero() {
			continue
		}

		if err := writeValidationWebhookFile(vwh, tpl, logger); err != nil {
			return fmt.Errorf("write validation webhook %s: %w", vwh.Name, err)
		}
	}

	return nil
}

// writeConversionWebhookFile renders the conversion webhook template and writes the
// resulting Python file to the hooks directory. It is idempotent: if the file
// already exists and has the same content, it is not rewritten.
func writeConversionWebhookFile(cwh *deckhouseiov1alpha1.ConversionWebhook, tpl string, logger *log.Logger) error {
	buf, err := templater.RenderConversionTemplate(tpl, cwh)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	dir := conversionWebhookDir(cwh.Name)
	if err := os.MkdirAll(dir, directoryPermissions); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}

	filePath := conversionWebhookFilePath(cwh.Name)

	if !isFileChanged(filePath, buf.Bytes()) {
		logger.Debug("presync: conversion webhook file already up-to-date", slog.String("webhook", cwh.Name))
		return nil
	}

	if err := os.WriteFile(filePath, buf.Bytes(), filePermissions); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
	}

	logger.Info("presync: wrote conversion webhook file", slog.String("webhook", cwh.Name), slog.String("path", filePath))
	return nil
}

// writeValidationWebhookFile renders the validation webhook template and writes the
// resulting Python file to the hooks directory. It is idempotent.
func writeValidationWebhookFile(vwh *deckhouseiov1alpha1.ValidationWebhook, tpl string, logger *log.Logger) error {
	buf, err := templater.RenderValidationTemplate(tpl, vwh)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	dir := validationWebhookDir(vwh.Name)
	if err := os.MkdirAll(dir, validationWebhookDirectoryPerms); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}

	filePath := validationWebhookFilePath(vwh.Name)

	if !isFileChanged(filePath, buf.Bytes()) {
		logger.Debug("presync: validation webhook file already up-to-date", slog.String("webhook", vwh.Name))
		return nil
	}

	if err := os.WriteFile(filePath, buf.Bytes(), validationWebhookFilePerms); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
	}

	logger.Info("presync: wrote validation webhook file", slog.String("webhook", vwh.Name), slog.String("path", filePath))
	return nil
}

// Package-level path helpers (same logic as the reconciler methods, but callable
// without a reconciler receiver so presync can use them).

func conversionWebhookDir(webhookName string) string {
	return filepath.Join(hooksBaseDir, webhookName, webhooksSubDir, conversionSubDir)
}

func conversionWebhookFilePath(webhookName string) string {
	return filepath.Join(conversionWebhookDir(webhookName), webhookName+webhookFileExtension)
}

func validationWebhookDir(webhookName string) string {
	return filepath.Join(validationHooksBaseDir, webhookName, validationWebhooksSubDir, validatingSubDir)
}

func validationWebhookFilePath(webhookName string) string {
	return filepath.Join(validationWebhookDir(webhookName), webhookName+validationWebhookFileExt)
}

// isFileChanged compares the file on disk with the rendered content.
// Returns true if the file does not exist or has different content.
func isFileChanged(filePath string, rendered []byte) bool {
	existing, err := os.ReadFile(filePath)
	if err != nil {
		return true
	}

	return string(existing) != string(rendered)
}
