// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package packagerepositoryoperation

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/registry"
)

func TestClassifyScanFailure(t *testing.T) {
	serverErrorURL, _ := url.Parse("https://registry.example.com/v2/test/tags/list")

	tests := map[string]struct {
		cause   error
		reason  string
		message string
	}{
		"access denied through the service wrappers": {
			cause:   fmt.Errorf("failed to list packages: failed to list tags: %w", errRegistryAccessDenied),
			reason:  v1alpha1.PackageRepositoryOperationReasonAccessDenied,
			message: "Registry rejected the credentials from spec.registry: check login/password or dockerCfg",
		},
		"repository not found": {
			cause:   fmt.Errorf("failed to list tags: %w", errRegistryRepositoryNotFound),
			reason:  v1alpha1.PackageRepositoryOperationReasonRepositoryNotFound,
			message: "Registry has no repository at spec.registry.repo: check the path",
		},
		"image not found is not a repository failure": {
			cause:   fmt.Errorf("failed to get image: %w", registry.ErrImageNotFound),
			reason:  v1alpha1.PackageRepositoryOperationReasonScanFailed,
			message: "failed to get image: image not found",
		},
		"server error with request": {
			cause: fmt.Errorf("failed to list tags: %w", &transport.Error{
				StatusCode: http.StatusServiceUnavailable,
				Request:    &http.Request{URL: serverErrorURL},
			}),
			reason:  v1alpha1.PackageRepositoryOperationReasonRegistryUnavailable,
			message: "Registry is unavailable: HTTP 503 from https://registry.example.com/v2/test/tags/list",
		},
		"server error without request": {
			cause:   &transport.Error{StatusCode: http.StatusBadGateway},
			reason:  v1alpha1.PackageRepositoryOperationReasonRegistryUnavailable,
			message: "Registry is unavailable: HTTP 502",
		},
		"connection refused": {
			cause:   fmt.Errorf("failed to list tags: %w", errRegistryUnreachable),
			reason:  v1alpha1.PackageRepositoryOperationReasonRegistryUnavailable,
			message: `Registry is unavailable: Get "https://registry.example.com/v2/": dial tcp 10.0.0.1:443: connect: connection refused`,
		},
		"context deadline": {
			cause:   fmt.Errorf("list tags: %w", context.DeadlineExceeded),
			reason:  v1alpha1.PackageRepositoryOperationReasonRegistryUnavailable,
			message: "Registry is unavailable: request timed out",
		},
		"unknown error keeps the full text": {
			cause:   fmt.Errorf("create package service: %w", assert.AnError),
			reason:  v1alpha1.PackageRepositoryOperationReasonScanFailed,
			message: "create package service: assert.AnError general error for testing",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			reason, message := classifyScanFailure(tc.cause)

			assert.Equal(t, tc.reason, reason)
			assert.Equal(t, tc.message, message)
		})
	}
}
