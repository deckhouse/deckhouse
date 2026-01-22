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

package client

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientWithOptions_InsecureFlag(t *testing.T) {
	t.Run("Insecure=true sets insecure flag", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set")
		assert.True(t, opts.Insecure, "opts.Insecure should remain true")
	})

	t.Run("Insecure=false keeps secure mode", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.False(t, client.insecure, "client should not have insecure flag set")
		assert.False(t, opts.Insecure, "opts.Insecure should remain false")
	})

	t.Run("Scheme=http sets insecure flag", func(t *testing.T) {
		opts := &Options{
			Scheme: "http",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set when Scheme=http")
		assert.True(t, opts.Insecure, "opts.Insecure should be set to true when Scheme=http")
	})

	t.Run("Scheme=HTTP (uppercase) sets insecure flag", func(t *testing.T) {
		opts := &Options{
			Scheme: "HTTP",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set when Scheme=HTTP")
		assert.True(t, opts.Insecure, "opts.Insecure should be set to true when Scheme=HTTP")
	})

	t.Run("Scheme=https keeps secure mode", func(t *testing.T) {
		opts := &Options{
			Scheme: "https",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.False(t, client.insecure, "client should not have insecure flag set when Scheme=https")
		assert.False(t, opts.Insecure, "opts.Insecure should remain false when Scheme=https")
	})

	t.Run("Insecure=true with Scheme=https keeps insecure", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
			Scheme:   "https",
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.True(t, client.insecure, "client should have insecure flag set when explicitly set")
		assert.True(t, opts.Insecure, "opts.Insecure should remain true")
	})

	t.Run("Default (no flags) uses secure mode", func(t *testing.T) {
		opts := &Options{}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.False(t, client.insecure, "client should default to secure mode")
		assert.False(t, opts.Insecure, "opts.Insecure should default to false")
	})
}

func TestClient_NameOptions(t *testing.T) {
	t.Run("insecure client returns name.Insecure option", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		nameOpts := client.nameOptions()
		require.Len(t, nameOpts, 1, "should return one name option")

		// Verify the option works by parsing a reference
		ref, err := name.ParseReference("registry.example.com/repo:tag", nameOpts...)
		require.NoError(t, err)
		assert.Equal(t, "http", ref.Context().Registry.Scheme(), "should use HTTP scheme")
	})

	t.Run("secure client returns no options", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		nameOpts := client.nameOptions()
		assert.Nil(t, nameOpts, "should return nil for secure client")

		// Verify default behavior uses HTTPS
		ref, err := name.ParseReference("registry.example.com/repo:tag")
		require.NoError(t, err)
		assert.Equal(t, "https", ref.Context().Registry.Scheme(), "should use HTTPS scheme by default")
	})
}

func TestClient_WithSegment_PreservesInsecure(t *testing.T) {
	t.Run("WithSegment preserves insecure flag", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		segmentedClient := client.WithSegment("deckhouse", "ee").(*Client)

		assert.True(t, segmentedClient.insecure, "WithSegment should preserve insecure flag")
		assert.Equal(t, "registry.example.com/deckhouse/ee", segmentedClient.GetRegistry())
	})

	t.Run("WithSegment preserves secure mode", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		segmentedClient := client.WithSegment("deckhouse").(*Client)

		assert.False(t, segmentedClient.insecure, "WithSegment should preserve secure mode")
	})
}

func TestClient_ParseReference_UsesInsecureOption(t *testing.T) {
	t.Run("insecure client parses references with HTTP scheme", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("localhost:5000", opts)

		// Test that nameOptions returns the correct option
		nameOpts := client.nameOptions()
		ref, err := name.ParseReference("localhost:5000/repo:tag", nameOpts...)
		require.NoError(t, err)

		assert.Equal(t, "http", ref.Context().Registry.Scheme(), "should parse with HTTP scheme")
		assert.Equal(t, "localhost:5000", ref.Context().RegistryStr())
	})

	t.Run("secure client parses references with HTTPS scheme", func(t *testing.T) {
		opts := &Options{
			Insecure: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		nameOpts := client.nameOptions()
		ref, err := name.ParseReference("registry.example.com/repo:tag", nameOpts...)
		require.NoError(t, err)

		assert.Equal(t, "https", ref.Context().Registry.Scheme(), "should parse with HTTPS scheme")
	})
}

func TestClient_TransportConfiguration(t *testing.T) {
	t.Run("insecure flag creates custom transport", func(t *testing.T) {
		opts := &Options{
			Insecure: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.NotNil(t, client.transport, "should create custom transport for insecure mode")
	})

	t.Run("TLSSkipVerify creates custom transport", func(t *testing.T) {
		opts := &Options{
			TLSSkipVerify: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.NotNil(t, client.transport, "should create custom transport for TLS skip verify")
	})

	t.Run("secure mode without TLS skip uses default transport", func(t *testing.T) {
		opts := &Options{
			Insecure:      false,
			TLSSkipVerify: false,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.Nil(t, client.transport, "should not create custom transport for default secure mode")
	})

	t.Run("both insecure and TLSSkipVerify create custom transport", func(t *testing.T) {
		opts := &Options{
			Insecure:      true,
			TLSSkipVerify: true,
		}
		client := NewClientWithOptions("registry.example.com", opts)

		assert.NotNil(t, client.transport, "should create custom transport when both flags are set")
	})
}
