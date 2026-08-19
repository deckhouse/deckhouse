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

package checks

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
)

type registryAuthRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f registryAuthRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCheckBasicRegistryAuthWithoutAPIVersionHeader(t *testing.T) {
	const authData = "dGVzdDp0ZXN0"

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
	}

	client := &http.Client{
		Transport: registryAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "Basic "+authData, req.Header.Get("Authorization"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		}),
	}

	err := checkBasicRegistryAuth(t.Context(), metaConfig, authData, client)
	require.NoError(t, err)
}
