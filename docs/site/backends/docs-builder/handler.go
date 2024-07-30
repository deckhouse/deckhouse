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
	"io"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/mux"
)

func newHandler(highAvailability bool) *mux.Router {
	var isReady atomic.Bool

	if !highAvailability {
		isReady.Store(true)
	}

	channelMappingEditor := newChannelMappingEditor(src)

	r := mux.NewRouter()
	r.Handle("/readyz", newReadinessHandler(&isReady))
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "OK") })
	r.Handle("/loadDocArchive/{moduleName}/{version}", newLoadHandler(src, channelMappingEditor)).Methods(http.MethodPost)
	r.Handle("/build", newBuildHandler(src, dst, &isReady, channelMappingEditor)).Methods(http.MethodPost)

	return r
}
