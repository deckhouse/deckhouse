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
)

func newReadinessHandler(isReady *atomic.Bool) *readinessHandler {
	return &readinessHandler{
		isReady: isReady,
	}
}

type readinessHandler struct {
	isReady *atomic.Bool
}

func (h *readinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.isReady.Load() {
		_, _ = io.WriteString(w, "ok")
	} else {
		http.Error(w, "Waiting for first build", http.StatusInternalServerError)
	}
}
