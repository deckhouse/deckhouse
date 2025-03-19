/*
Copyright 2023 Flant JSC

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

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/otiai10/copy"
)

func installFileIfChanged(src, dst string, perm os.FileMode) error {
	var srcBytes, dstBytes []byte

	src, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	srcBytes, err = os.ReadFile(src)
	if err != nil {
		return err
	}

	dstBytes, _ = os.ReadFile(dst)

	srcBytes = []byte(os.ExpandEnv(string(srcBytes)))

	if bytes.Equal(srcBytes, dstBytes) {
		log.Info("file is not changed, skipping", slog.String("path", dst))
		return nil
	}

	if err := backupFile(dst); err != nil {
		log.Warn("Backup failed", log.Err(err))
	}

	log.Info("install file to destination", slog.String("src", src), slog.String("destination", dst))
	if err := os.WriteFile(dst, srcBytes, perm); err != nil {
		return err
	}

	return os.Chown(dst, 0, 0)
}

func backupFile(src string) error {
	log.Info("backup file", slog.String("path", src))

	if _, err := os.Stat(src); err != nil {
		return err
	}

	backupDir := filepath.Join(deckhousePath, "backup", fmt.Sprintf("%d-%02d-%02d_%s", nowTime.Year(), nowTime.Month(), nowTime.Day(), config.ConfigurationChecksum))

	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}
	return copy.Copy(src, backupDir+src)
}

func removeFile(src string) error {
	log.Info("remove file", slog.String("path", src))
	if err := backupFile(src); err != nil {
		return err
	}
	return os.Remove(src)
}

func removeDirectory(dir string) error {
	walkDirFunc := func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		return removeFile(path)
	}

	err := filepath.WalkDir(dir, walkDirFunc)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func removeOrphanFiles() {
	srcDir := filepath.Join(deckhousePath, "kubeadm", "patches")
	log.Info("phase: remove orphan files from dir", slog.String("dir", srcDir))

	walkDirFunc := func(path string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() {
			return nil
		}

		switch _, file := filepath.Split(path); file {
		case "kube-apiserver.yaml":
			return nil
		case "etcd.yaml":
			return nil
		case "kube-controller-manager.yaml":
			return nil
		case "kube-scheduler.yaml":
			return nil
		default:
			return removeFile(path)
		}
	}

	if err := filepath.WalkDir(srcDir, walkDirFunc); err != nil {
		log.Warn(err.Error())
	}
}

func stringSlicesEqual(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)

	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func removeOldBackups() error {
	backupPath := filepath.Join(deckhousePath, "backup")
	log.Info("remove backups older than 5")
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return err
	}
	files := make([]fs.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return err
		}
		files = append(files, info)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})

	if len(files) <= 5 {
		return nil
	}
	for _, f := range files[5:] {
		log.Info("removing old backup dir", slog.String("dir", f.Name()))
		if err := os.RemoveAll(filepath.Join(backupPath, f.Name())); err != nil {
			return err
		}
	}
	return nil
}

func cleanup() {
	if err := os.RemoveAll(config.TmpPath); err != nil {
		log.Warn(err.Error())
	}

	if err := removeOldBackups(); err != nil {
		log.Warn(err.Error())
	}
}
