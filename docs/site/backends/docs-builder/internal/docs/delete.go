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
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

func newDeleteHandler(baseDir string, channelMappingEditor *channelMappingEditor) *deleteHandler {
	return &deleteHandler{baseDir: baseDir, channelMappingEditor: channelMappingEditor}
}

type deleteHandler struct {
	baseDir string

	channelMappingEditor *channelMappingEditor
}

func (d *deleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	channelsStr := r.URL.Query().Get("channels")
	channels := []string{"stable"}
	if len(channelsStr) != 0 {
		channels = strings.Split(channelsStr, ",")
	}

	pathVars := mux.Vars(r)
	moduleName := pathVars["moduleName"]

	klog.Infof("deleting %s: %s", moduleName, channels)
	err := d.delete(moduleName, channels)
	if err != nil {
		klog.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = d.removeFromChannelMapping(moduleName)
	if err != nil {
		klog.Error(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (d *deleteHandler) delete(moduleName string, channels []string) error {
	for _, channel := range channels {
		path := filepath.Join(d.baseDir, "content/modules", moduleName, channel)
		err := os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}

		path = filepath.Join(d.baseDir, "data/modules", moduleName, channel)
		err = os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}

	return nil
}

func (d *deleteHandler) removeFromChannelMapping(moduleName string) error {
	return d.channelMappingEditor.edit(func(m channelMapping) {
		delete(m, moduleName)
	})
}
