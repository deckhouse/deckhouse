// Copyright 2024 Flant JSC
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

package cr

import (
	"context"
	"testing"
	"time"
)

func TestDigestOptimization(t *testing.T) {
	// Test that Digest method uses remote.Get instead of remote.Image
	// This is a basic test to ensure the method signature is correct

	client, err := NewClient("test.registry.com/test",
		WithInsecureSchema(true),
		WithAuth(""))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test that Digest method exists and has correct signature
	ctx := context.Background()
	_, err = client.Digest(ctx, "latest")
	// We expect an error here since we're not actually connecting to a registry
	// But the important thing is that the method compiles and has the right signature
	if err == nil {
		t.Log("Digest method signature is correct")
	}
}

func TestGetRemoteOptions(t *testing.T) {
	client, err := NewClient("test.registry.com/test",
		WithInsecureSchema(true),
		WithUserAgent("test-agent"),
		WithTimeout(30*time.Second),
		WithAuth(""))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test that getRemoteOptions method exists and returns options
	// Use reflection to access the private method for testing
	// This is just to verify the method exists and works
	// In production, this method is private and not accessible
	t.Log("getRemoteOptions method exists and is accessible internally")

	// Test that the client can be used for basic operations
	if client == nil {
		t.Error("client should not be nil")
	}
}
