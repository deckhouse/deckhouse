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

package dvp

import (
	"context"
	"testing"
)

func TestInstanceExistsByProviderIDSkipsForeignProvider(t *testing.T) {
	// A nil dvpService is intentional: foreign provider IDs must return before
	// any request to the DVP API is attempted.
	cloud := &Cloud{}

	for _, providerID := range []string{"other://instance", "", "dvp://"} {
		t.Run(providerID, func(t *testing.T) {
			exists, err := cloud.InstanceExistsByProviderID(context.Background(), providerID)
			if err != nil {
				t.Fatalf("InstanceExistsByProviderID() returned an unexpected error: %v", err)
			}
			if !exists {
				t.Fatal("InstanceExistsByProviderID() returned false for an unmanaged provider ID")
			}
		})
	}
}

func TestInstanceShutdownByProviderIDSkipsForeignProvider(t *testing.T) {
	// A nil dvpService is intentional: foreign provider IDs must return before
	// any request to the DVP API is attempted.
	cloud := &Cloud{}

	shutdown, err := cloud.InstanceShutdownByProviderID(context.Background(), "other://instance")
	if err != nil {
		t.Fatalf("InstanceShutdownByProviderID() returned an unexpected error: %v", err)
	}
	if shutdown {
		t.Fatal("InstanceShutdownByProviderID() returned true for a foreign provider")
	}
}
