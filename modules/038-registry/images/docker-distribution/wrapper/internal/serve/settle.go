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

package serve

import "net/http"

// settle keeps a response that has started from being answered a second time.
//
// Distribution answers a cache miss by streaming the blob to the client and into the store at once,
// and reports whatever went wrong with the store copy only after the blob is out: a status and an
// error body, written into a response whose status and length were already given. A store that is
// full drops every copy — see bound.Discard — so at the edge that is every miss.
//
// Over HTTP/2 it is not harmless. The stream refuses a write beyond the declared length only once
// its headers have gone out, and for a body smaller than its buffer they have not: the error lands
// in the same frames as the blob, the response arrives longer than it said, and the node agent's
// transport drops it as interrupted — the client already has its 200, so no fallback is tried, and
// the pull fails with EOF. A live cluster found this on a two-kilobyte image config.
//
// So once the body has started, a later status is the second answer, and it and everything after it
// are dropped. The error is still logged by distribution; the client gets exactly the blob it was
// promised. A normal response never writes a status after its body, so nothing else is affected.
func settle(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handler.ServeHTTP(&settled{ResponseWriter: writer}, request)
	})
}

type settled struct {
	http.ResponseWriter

	// body is set once a byte of the body has been written, done once a status came after it.
	body, done bool
}

func (s *settled) WriteHeader(code int) {
	if s.body {
		s.done = true
		return
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *settled) Write(p []byte) (int, error) {
	if s.done {
		return len(p), nil
	}
	if len(p) > 0 {
		s.body = true
	}
	return s.ResponseWriter.Write(p)
}

// Flush and Unwrap keep what distribution and net/http find by asserting on the writer.
func (s *settled) Flush() {
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok && !s.done {
		flusher.Flush()
	}
}

func (s *settled) Unwrap() http.ResponseWriter { return s.ResponseWriter }
