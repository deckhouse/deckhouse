// Copyright 2021 Flant JSC
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

package actions

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestManifestTaskCreateOrUpdateSilentIncludesResourceNameOnCreateError(t *testing.T) {
	task := &ManifestTask{
		Name: "Secret d8-system/bad-secret",
		Manifest: func() interface{} {
			return nil
		},
		CreateFunc: func(ctx context.Context, manifest interface{}) error {
			return errors.New(`Secret in version "v1" cannot be handled as a Secret: illegal base64 data at input byte 2204`)
		},
		UpdateFunc: func(ctx context.Context, manifest interface{}) error {
			t.Fatal("update should not be called")
			return nil
		},
	}

	err := task.CreateOrUpdateSilent(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), `create resource "Secret d8-system/bad-secret"`) {
		t.Fatalf("expected error to contain resource name, got: %v", err)
	}

	if !strings.Contains(err.Error(), "illegal base64 data") {
		t.Fatalf("expected original error to be preserved, got: %v", err)
	}
}
