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
	"os"
	"path/filepath"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// backupStep creates a per-component backup of files.
// Now it is always the first command in every pipeline.
// Writes directly into per-operation final directory.
type backupStep struct{}

func (c *backupStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
	component := env.State.Raw().Spec.Component
	operationName := env.State.Raw().Name
	files := backupFilesForComponent(component, env.Node.KubeconfigDir)

	componentBackupDir := filepath.Join(constants.BackupBasePath, string(component))
	finalDir := filepath.Join(componentBackupDir, operationName)
	if err := os.RemoveAll(finalDir); err != nil {
		return reconcile.Result{}, fmt.Errorf("clean previous backup dir on command re-execution: %w", err)
	}

	wasBackupped := false
	for _, src := range files {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		rel, err := filepath.Rel(constants.KubernetesConfigPath, src)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("relative path for %s: %w", src, err)
		}
		dst := filepath.Join(finalDir, rel)

		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return reconcile.Result{}, fmt.Errorf("create backup dir: %w", err)
		}
		if err := copyFile(src, dst); err != nil {
			return reconcile.Result{}, fmt.Errorf("backup %s: %w", rel, err)
		}
		// logger.Info("backup file", slog.String("file", rel))
		wasBackupped = true
	}

	if !wasBackupped {
		logger.Info("no files to back up for component", slog.String("component", string(component)))
		return reconcile.Result{}, nil
	}

	if err := rotateBackups(componentBackupDir, constants.MaxBackupsPerComponent); err != nil {
		logger.Warn("failed to rotate backups", log.Err(err))
	}

	return reconcile.Result{}, nil
}

// backupFilesForComponent returns the list of absolute file paths that should be backup.
// Static pod manifest, leaf certs, CA, kubeconfigs, extra files.
func backupFilesForComponent(component controlplanev1alpha1.OperationComponent, kubeconfigDir string) []string {
	deps := componentDeps(component)
	var files []string

	if name := component.PodComponentName(); name != "" {
		files = append(files, filepath.Join(constants.ManifestsPath, name+".yaml"))
	}

	for _, leafName := range deps.leafCertFiles() {
		baseName := string(leafName)
		files = append(files,
			filepath.Join(constants.KubernetesPkiPath, baseName+".crt"),
			filepath.Join(constants.KubernetesPkiPath, baseName+".key"),
		)
	}

	for _, relPath := range deps.CAFiles {
		files = append(files, filepath.Join(constants.KubernetesPkiPath, relPath))
	}

	for _, kf := range deps.KubeconfigFiles {
		files = append(files, filepath.Join(kubeconfigDir, string(kf)))
	}

	for _, key := range deps.ExtraFileKeys {
		files = append(files, filepath.Join(constants.ExtraFilesPath, strings.TrimPrefix(key, "extra-file-")))
	}

	return files
}

// rotateBackups keeps only the N most recent backup directories under componentBackupDir.
func rotateBackups(componentBackupDir string, keep int) error {
	return rotateDirectories(componentBackupDir, keep)
}
