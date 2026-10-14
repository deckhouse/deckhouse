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

package controlplaneoperation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckReadyz(t *testing.T) {
	t.Parallel()

	var status atomic.Int32
	status.Store(http.StatusServiceUnavailable)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/readyz", r.URL.Path)
		w.WriteHeader(int(status.Load()))
	}))
	defer srv.Close()

	ctx := context.Background()
	require.ErrorContains(t, checkReadyz(ctx, readyzClient, srv.URL+"/readyz"), "returned 503")

	status.Store(http.StatusOK)
	require.NoError(t, checkReadyz(ctx, readyzClient, srv.URL+"/readyz"))

	srv.Close()
	require.Error(t, checkReadyz(ctx, readyzClient, srv.URL+"/readyz"))
}
