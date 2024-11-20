// Copyright 2023 Flant JSC
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

package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/fsync"
	"k8s.io/klog/v2"

	"github.com/flant/docs-builder/pkg/hugo"
)

func (svc *Service) Build() error {
	err := svc.buildHugo()
	if err != nil {
		svc.isReady.Store(false)

		return fmt.Errorf("hugo build: %w", err)
	}

	for _, lang := range []string{"ru", "en"} {
		glob := filepath.Join(svc.destDir, "public", lang, "modules/*")
		err = removeGlob(glob)
		if err != nil {
			return fmt.Errorf("clear %s: %w", svc.destDir, err)
		}

		oldLocation := filepath.Join(svc.baseDir, "public", lang, "modules")
		newLocation := filepath.Join(svc.destDir, "public", lang, "modules")
		err = fsync.Sync(newLocation, oldLocation)
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", oldLocation, newLocation, err)
		}
	}

	svc.isReady.Store(true)

	return nil
}

func (svc *Service) buildHugo() error {
	flags := hugo.Flags{
		LogLevel: "debug",
		Source:   svc.baseDir,
		CfgDir:   filepath.Join(svc.baseDir, "config"),
	}

	for {
		err := hugo.Build(flags)
		if err == nil {
			return nil
		}

		if path, ok := getAssembleErrorPath(err.Error()); ok {
			modulePath := filepath.Dir(path)
			err = os.RemoveAll(modulePath)
			if err != nil {
				return fmt.Errorf("remove module: %w", err)
			}

			moduleName, channel := parseModulePath(modulePath)
			err = svc.removeModuleFromChannelMapping(moduleName, channel)
			if err != nil {
				return fmt.Errorf("remove module from channel mapping: %w", err)
			}

			klog.Warningf("removed broken module %q", modulePath)
			continue
		}

		return err
	}
}

func (svc *Service) removeModuleFromChannelMapping(moduleName, channel string) error {
	return svc.channelMappingEditor.edit(func(m channelMapping) {
		delete(m[moduleName]["channels"], channel)
	})
}

func getAssembleErrorPath(errorMessage string) (string, bool) {
	match := assembleErrorRegexp.FindStringSubmatch(errorMessage)
	if len(match) == 6 {
		return match[2], true
	}

	return "", false
}

func parseModulePath(modulePath string) (moduleName, channel string) {
	s := strings.Split(modulePath, "/")
	if len(s) < 2 {
		klog.Error("failed to parse", modulePath)
		return "", ""
	}

	return s[len(s)-2], s[len(s)-1]
}

func removeGlob(path string) error {
	contents, err := filepath.Glob(path)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	for _, item := range contents {
		err = os.RemoveAll(item)
		if err != nil {
			return fmt.Errorf("remove all: %w", err)
		}
	}

	return nil
}
