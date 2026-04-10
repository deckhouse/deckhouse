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
	"sort"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// backupCommand creates a per-component backup of files.
// Now it is always the first command in every pipeline.
// Write into tmpDir, promote to finalDir via rename only on success.
type backupCommand struct{}

func (c *backupCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	component := env.State.Raw().Spec.Component
	operationName := env.State.Raw().Name
	files := backupFilesForComponent(component, env.Node.KubeconfigDir)

	componentBackupDir := filepath.Join(constants.BackupBasePath, string(component))
	tmpDir := filepath.Join(componentBackupDir, backupTmpPrefix+operationName)

	// Remove any leftover tmp from a previously crashed attempt of this operation.
	// Other operations tmp dirs not affected.
	if err := os.RemoveAll(tmpDir); err != nil {
		return reconcile.Result{}, fmt.Errorf("clean previous tmp backup: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	wasBackupped := false
	for _, src := range files {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		rel, err := filepath.Rel(constants.KubernetesConfigPath, src)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("relative path for %s: %w", src, err)
		}
		dst := filepath.Join(tmpDir, rel)

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
		success = true
		return reconcile.Result{}, nil
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
	finalDir := filepath.Join(componentBackupDir, fmt.Sprintf("%s__%s", timestamp, operationName))
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return reconcile.Result{}, fmt.Errorf("finalize backup: %w", err)
	}
	success = true

	if err := rotateBackups(componentBackupDir, constants.MaxBackupsPerComponent); err != nil {
		logger.Warn("failed to rotate backups", log.Err(err))
	}

	return reconcile.Result{}, nil
}

// backupTmpPrefix marks in-progress per-operation tmp directories under a component backup dir.
const backupTmpPrefix = ".tmp-"

// backupFilesForComponent returns the list of absolute file paths that should be backup.
// Static pod manifest, leaf certs, CA, kubeconfigs, extra files, hot-reload files.
func backupFilesForComponent(component controlplanev1alpha1.OperationComponent, kubeconfigDir string) []string {
	var files []string

	if name := component.PodComponentName(); name != "" {
		files = append(files, filepath.Join(constants.ManifestsPath, name+".yaml"))
	}

	for _, baseName := range componentLeafCertFiles[component] {
		files = append(files,
			filepath.Join(constants.KubernetesPkiPath, baseName+".crt"),
			filepath.Join(constants.KubernetesPkiPath, baseName+".key"),
		)
	}

	for _, relPath := range componentCAFiles[component] {
		files = append(files, filepath.Join(constants.KubernetesPkiPath, relPath))
	}

	for _, kf := range kubeconfigFilesForComponent(component) {
		files = append(files, filepath.Join(kubeconfigDir, string(kf)))
	}

	if podName := component.PodComponentName(); podName != "" {
		for _, key := range checksum.ExtraFileKeysForPodComponent(podName) {
			files = append(files, filepath.Join(constants.ExtraFilesPath, strings.TrimPrefix(key, "extra-file-")))
		}
	}

	if component == controlplanev1alpha1.OperationComponentHotReload {
		for _, key := range checksum.HotReloadChecksumDependsOn {
			files = append(files, filepath.Join(constants.ExtraFilesPath, strings.TrimPrefix(key, "extra-file-")))
		}
	}

	return files
}

// rotateBackups keeps only the N most recent completed backup directories under componentBackupDir.
// In-progress tmp directories (prefix backupTmpPrefix) are ignored.
func rotateBackups(componentBackupDir string, keep int) error {
	entries, err := os.ReadDir(componentBackupDir)
	if err != nil {
		return fmt.Errorf("read backup dir: %w", err)
	}

	type backupEntry struct {
		name  string
		mtime time.Time
	}
	var backups []backupEntry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), backupTmpPrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("stat backup %s: %w", e.Name(), err)
		}
		backups = append(backups, backupEntry{name: e.Name(), mtime: info.ModTime()})
	}

	if len(backups) <= keep {
		return nil
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].mtime.After(backups[j].mtime)
	})

	for _, b := range backups[keep:] {
		path := filepath.Join(componentBackupDir, b.name)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove old backup %s: %w", b.name, err)
		}
	}

	return nil
}
