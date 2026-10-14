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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSuiteSPEInventory(t *testing.T) {
	// Any directory inside the chart resolves the module CRD.
	matrixDir, err := filepath.Abs(filepath.Join("..", "..", "test_cases", "constraints", "security", "allowed-sysctls"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		spec    string
		wantErr string
	}{
		{
			name: "valid",
			spec: "securityContext:\n    sysctls:\n      allowedValues:\n      - name: net.core.rmem_max\n",
		},
		{
			name:    "wrong type",
			spec:    "securityContext:\n    sysctls:\n      allowedValues:\n      - net.core.rmem_max\n",
			wantErr: "must be of type object",
		},
		{
			name:    "unknown field",
			spec:    "volumes:\n    hostPath:\n      allowedValues:\n      - path: /opt\n        readonly: true\n",
			wantErr: "spec.volumes.hostPath.allowedValues[0].readonly: field not declared in the schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			doc := "apiVersion: deckhouse.io/v1alpha1\nkind: SecurityPolicyException\nmetadata:\n  name: test\n  namespace: testns\nspec:\n  " + tt.spec
			if err := os.WriteFile(filepath.Join(baseDir, "spe.yaml"), []byte(doc), 0o644); err != nil {
				t.Fatal(err)
			}
			suite := suiteOut{Tests: []testBlockOut{{Cases: []caseOut{{Inventory: []string{"spe.yaml"}}}}}}

			err := validateSuiteSPEInventory(suite, baseDir, matrixDir)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("expected an error containing %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
