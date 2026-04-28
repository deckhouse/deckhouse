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

package rpp

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDigest(t *testing.T) {
	tests := []struct {
		name    string
		digest  string
		wantErr bool
	}{
		{name: "valid sha256", digest: "sha256:abc123def456"},
		{name: "valid sha512", digest: "sha512:0123456789abcdef"},
		{name: "missing colon", digest: "sha256abc123", wantErr: true},
		{name: "uppercase algorithm", digest: "SHA256:abc123", wantErr: true},
		{name: "uppercase hash", digest: "sha256:ABC123", wantErr: true},
		{name: "empty string", digest: "", wantErr: true},
		{name: "whitespace around", digest: " sha256:abc123 ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDigest(tt.digest)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestBuildPackageURL(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		digest     string
		repository string
		path       string
		wantURL    string
	}{
		{
			name:     "digest only",
			endpoint: "1.2.3.4:4219",
			digest:   "sha256:abc123",
			wantURL:  "https://1.2.3.4:4219/package?digest=sha256%3Aabc123",
		},
		{
			name:     "endpoint with https scheme",
			endpoint: "https://1.2.3.4:4219",
			digest:   "sha256:abc123",
			wantURL:  "https://1.2.3.4:4219/package?digest=sha256%3Aabc123",
		},
		{
			name:     "endpoint with http scheme",
			endpoint: "http://1.2.3.4:4219",
			digest:   "sha256:abc123",
			wantURL:  "https://1.2.3.4:4219/package?digest=sha256%3Aabc123",
		},
		{
			name:       "with repository",
			endpoint:   "1.2.3.4:4219",
			digest:     "sha256:abc123",
			repository: "myrepo",
			wantURL:    "https://1.2.3.4:4219/package?digest=sha256%3Aabc123&repository=myrepo",
		},
		{
			name:     "with path",
			endpoint: "1.2.3.4:4219",
			digest:   "sha256:abc123",
			path:     "/custom/path",
			wantURL:  "https://1.2.3.4:4219/package?digest=sha256%3Aabc123&path=%2Fcustom%2Fpath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPackageURL(tt.endpoint, tt.digest, tt.repository, tt.path)
			assert.Equal(t, tt.wantURL, got)
		})
	}
}

func TestShouldRetryFetch(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{name: "nil error", err: nil, wantRetry: false},
		{name: "context canceled", err: context.Canceled, wantRetry: false},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, wantRetry: false},
		{name: "invalid digest", err: errInvalidDigest, wantRetry: false},
		{name: "no endpoints", err: errNoEndpoints, wantRetry: false},
		{name: "no token", err: errNoToken, wantRetry: false},
		{
			name:      "http 408 request timeout",
			err:       &httpStatusError{statusCode: http.StatusRequestTimeout},
			wantRetry: true,
		},
		{
			name:      "http 429 too many requests",
			err:       &httpStatusError{statusCode: http.StatusTooManyRequests},
			wantRetry: true,
		},
		{
			name:      "http 500 internal server error",
			err:       &httpStatusError{statusCode: http.StatusInternalServerError},
			wantRetry: true,
		},
		{
			name:      "http 503 service unavailable",
			err:       &httpStatusError{statusCode: http.StatusServiceUnavailable},
			wantRetry: true,
		},
		{
			name:      "http 404 not found",
			err:       &httpStatusError{statusCode: http.StatusNotFound},
			wantRetry: false,
		},
		{
			name:      "http 401 unauthorized",
			err:       &httpStatusError{statusCode: http.StatusUnauthorized},
			wantRetry: false,
		},
		{
			name:      "generic network error",
			err:       errors.New("connection refused"),
			wantRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetryFetch(tt.err)
			assert.Equal(t, tt.wantRetry, got)
		})
	}
}
