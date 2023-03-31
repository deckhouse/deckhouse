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
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

func installExtraFiles() error {
	dstDir := filepath.Join(deckhousePath, "extra-files")
	log.Infof("install extra files to %s", dstDir)

	if err := removeDirectory(dstDir); err != nil {
		return err
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(configPath)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "extra-file-") {
			continue
		}

		if err := installFileIfChanged(filepath.Join(configPath, entry.Name()), filepath.Join(dstDir, strings.TrimPrefix(entry.Name(), "extra-file-")), 0644); err != nil {
			return err
		}
	}
	return nil
}
