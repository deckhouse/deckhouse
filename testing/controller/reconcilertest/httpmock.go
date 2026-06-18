// Copyright 2025 Flant JSC
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

package reconcilertest

import (
	"net/http"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// RespondHTTPOK configures the shared test dependency container's HTTP client to
// answer every request with a 200 OK. This is the most common expectation across
// controller suites (module readiness probes, doc builder pings, ...).
func RespondHTTPOK() {
	dependency.TestDC.HTTPClient.DoMock.
		Expect(&http.Request{}).
		Return(&http.Response{StatusCode: http.StatusOK}, nil)
}
