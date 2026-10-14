// Copyright 2026 Flant JSC
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

package proxy

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

// servedProxy starts Serve once per test binary: it registers its routes on
// http.DefaultServeMux, which panics on a second registration.
var servedProxy = sync.OnceValues(func() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	getter := &fakeCLIGetter{cfg: &registry.ClientConfig{Repository: "registry.test/deckhouse", Scheme: "https"}}
	p := NewProxy(&http.Server{ReadHeaderTimeout: time.Second}, listener, getter, nopCLILogger{}, &fakeCLIRegistryClient{})
	go p.Serve(nil)
	return "http://" + listener.Addr().String(), nil
})

// The requester of an extension declares its kind, so nobody asks the proxy what an
// image is: /package/labels is a path it does not serve.
func TestServe_packageLabelsIsNotFound(t *testing.T) {
	address, err := servedProxy()
	require.NoError(t, err)

	resp, err := http.Get(address + "/package/labels?digest=sha256:" + strings.Repeat("a", 64))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
