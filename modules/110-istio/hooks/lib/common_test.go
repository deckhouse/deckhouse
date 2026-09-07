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

package lib

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripClient func(*http.Request) (*http.Response, error)

func (f roundTripClient) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHTTPGetRejectsOversizedResponse(t *testing.T) {
	client := roundTripClient(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxHTTPResponseBodySize+1))),
		}, nil
	})

	if _, _, err := HTTPGet(client, "https://example.com", ""); err == nil {
		t.Fatal("HTTPGet must reject an oversized response body")
	}
}
