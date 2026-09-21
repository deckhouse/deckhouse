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

package service_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	"github.com/deckhouse/deckhouse/pkg/registry/client"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// listFailure makes a client whose tag listing fails with err. Embedding the
// interface leaves every other method as the wrapped client's, so only the one
// under test is replaced.
type listFailure struct {
	registry.Client

	err error
}

func (c listFailure) ListTags(context.Context, ...registry.ListTagsOption) ([]string, error) {
	return nil, c.err
}

func failingService(t *testing.T, err error) *service.BasicService {
	t.Helper()

	// The real client is used for the path it reports; no request is made,
	// since the one method the test drives is the overridden one.
	inner := client.New("registry.example.com").WithSegment("root")

	return service.NewBasicService("test", listFailure{Client: inner, err: err}, log.NewNop())
}

// transportError builds the answer a registry gives, with or without the OCI
// diagnostic body — a HEAD response never carries one, since HTTP forbids it.
func transportError(status int, codes ...transport.ErrorCode) *transport.Error {
	e := &transport.Error{StatusCode: status}

	for _, code := range codes {
		e.Errors = append(e.Errors, transport.Diagnostic{Code: code})
	}

	return e
}

// TestListTagsClassification covers what a listing failure is reported as.
//
// The client this package is built on classifies its own failures, so the
// sentinel has to survive the wrapping rather than be replaced by a flatter
// one; a client that classifies nothing gets the transport-level fallback. The
// two are checked side by side because they must agree.
func TestListTagsClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error

		notFound     bool
		repoNotFound bool
		denied       bool
	}{
		{
			name:         "client-classified missing repository",
			err:          fmt.Errorf("%w: %w", registry.ErrRepositoryNotFound, transportError(http.StatusNotFound, transport.NameUnknownErrorCode)),
			notFound:     true,
			repoNotFound: true,
		},
		{
			name:     "client-classified missing manifest",
			err:      fmt.Errorf("%w: %w", registry.ErrImageNotFound, transportError(http.StatusNotFound)),
			notFound: true,
		},
		{
			name:   "client-classified denial",
			err:    fmt.Errorf("%w: %w", registry.ErrAccessDenied, transportError(http.StatusForbidden)),
			denied: true,
		},
		{
			name:         "unclassified NAME_UNKNOWN",
			err:          transportError(http.StatusNotFound, transport.NameUnknownErrorCode),
			notFound:     true,
			repoNotFound: true,
		},
		{
			name:     "unclassified bare 404",
			err:      transportError(http.StatusNotFound),
			notFound: true,
		},
		{
			// A token-auth registry hides existence behind a 404 as readily as
			// a 403. Reporting that as "not found" would tell a caller the
			// repository is empty when the credential is simply wrong.
			name: "unclassified 404 carrying DENIED",
			err:  transportError(http.StatusNotFound, transport.DeniedErrorCode),
		},
		{
			name: "unclassified 401",
			err:  transportError(http.StatusUnauthorized),
		},
		{
			name: "unrelated failure",
			err:  errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := failingService(t, tt.err).ListTags(t.Context())
			require.Error(t, err)

			assert.Equal(t, tt.notFound, service.IsNotFound(err), "IsNotFound")
			assert.Equal(t, tt.repoNotFound, errors.Is(err, service.ErrRepositoryNotFound), "ErrRepositoryNotFound")
			assert.Equal(t, tt.denied, service.IsAccessDenied(err), "IsAccessDenied")

			// Whatever the classification, the original answer is still in the
			// chain: "not found" that hides the HTTP status is a bad diagnostic.
			assert.ErrorIs(t, err, tt.err, "original error must survive wrapping")
			assert.Contains(t, err.Error(), "registry.example.com/root")
		})
	}
}
