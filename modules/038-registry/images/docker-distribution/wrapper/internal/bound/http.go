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

package bound

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler refuses a push up front while the store is full, and passes everything else on.
//
// In front of the write endpoint, because that is the one place where the refusal is somebody's to
// read: an operator running `d8 mirror push`, or the syncer filling the leader. Left to the storage,
// the same refusal reaches the client as distribution's `500 UNKNOWN`, which says nothing and invites
// a retry. Here it is `507 Insufficient Storage` with the reason and the way out, before a byte of
// the body is read.
//
// Only what adds to the store is refused: starting, continuing and finishing an upload, and putting
// a manifest. A pull, a HEAD before a push, a deletion — everything else — is served as usual. A
// write the guard would still admit here can be refused further in, by the storage, when a blob
// turns out bigger than what is left; that one is the 500.
//
// Only a request that carries credentials is refused here. One without them goes on to be
// challenged by distribution as usual, and meets this refusal on the retry with a token: the state
// of a node's disk is not something to tell a client that has not said who it is.
func Handler(guard *Guard, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if adds(request) && request.Header.Get("Authorization") != "" {
			if full := guard.Refusal(); full != nil {
				refuse(writer, full)
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func adds(request *http.Request) bool {
	path := request.URL.Path
	switch request.Method {
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		if strings.Contains(path, "/blobs/uploads/") {
			return true
		}
	}
	return request.Method == http.MethodPut && strings.Contains(path, "/manifests/")
}

// refuse answers in the registry's own error format, so that a client prints the message rather
// than a status line.
func refuse(writer http.ResponseWriter, full *ErrStoreFull) {
	body := map[string]any{
		"errors": []map[string]any{{
			// No code in the distribution specification says "full"; UNKNOWN is the one every
			// client accepts, and the message is what carries the meaning.
			"code":    "UNKNOWN",
			"message": full.Error(),
			"detail": map[string]any{
				"reason":  string(full.Reason),
				"used":    full.Used,
				"budget":  full.Budget,
				"free":    full.Free,
				"reserve": full.Reserve,
			},
		}},
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusInsufficientStorage)
	_ = json.NewEncoder(writer).Encode(body)
}
