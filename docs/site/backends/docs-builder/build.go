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

package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/spf13/fsync"
	"k8s.io/klog/v2"

	"github.com/flant/docs-builder/pkg/hugo"
)

var assembleErrorRegexp = regexp.MustCompile(`error building site: assemble: (\x1b\[1;36m)?"(?P<path>.+):(?P<line>\d+):(?P<column>\d+)"(\x1b\[0m)?:`)

func newBuildHandler(src, dst string, wasCalled *atomic.Bool, channelMappingEditor *channelMappingEditor) *buildHandler {
	return &buildHandler{
		src:                  src,
		dst:                  dst,
		wasCalled:            wasCalled,
		channelMappingEditor: channelMappingEditor,
	}
}

type buildHandler struct {
	src                  string
	dst                  string
	wasCalled            *atomic.Bool
	channelMappingEditor *channelMappingEditor
}

func (b *buildHandler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	err := b.build()
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func (b *buildHandler) build() error {
	err := b.buildHugo()
	if err != nil {
		return fmt.Errorf("hugo build: %w", err)
	}

	for _, lang := range []string{"ru", "en"} {
		glob := filepath.Join(b.dst, "public", lang, "modules/*")
		err = removeGlob(glob)
		if err != nil {
			return fmt.Errorf("clear %s: %w", b.dst, err)
		}

		oldLocation := filepath.Join(b.src, "public", lang, "modules")
		newLocation := filepath.Join(b.dst, "public", lang, "modules")
		err = fsync.Sync(newLocation, oldLocation)
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", oldLocation, newLocation, err)
		}
	}

	b.wasCalled.Store(true)
	return nil
}

func (b *buildHandler) buildHugo() error {
	flags := hugo.Flags{
		LogLevel: "debug",
		Source:   b.src,
		CfgDir:   filepath.Join(b.src, "config"),
	}

	for {
		err := hugo.Build(flags)
		if err == nil {
			return nil
		}

		if path, ok := getAssembleErrorPath(err.Error()); ok {
			modulePath := getModulePath(path)
			err = os.RemoveAll(modulePath)
			if err != nil {
				return fmt.Errorf("remove module: %w", err)
			}

			moduleName, channel := parseModulePath(modulePath)
			err = b.removeModuleFromChannelMapping(moduleName, channel)
			if err != nil {
				return fmt.Errorf("remove module from channel mapping: %w", err)
			}

			klog.Warningf("removed broken module %q", modulePath)
			continue
		}

		return err
	}
}

func (b *buildHandler) removeModuleFromChannelMapping(moduleName, channel string) error {
	return b.channelMappingEditor.edit(func(m channelMapping) {
		delete(m[moduleName]["channels"], channel)
	})
}

func getAssembleErrorPath(errorMessage string) (string, bool) {
	match := assembleErrorRegexp.FindStringSubmatch(errorMessage)
	if match != nil && len(match) == 6 {
		return match[2], true
	}

	return "", false
}

func getModulePath(filePath string) string {
	return filepath.Dir(filePath)
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
