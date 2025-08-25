/*
Copyright 2025 Flant JSC

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

package main

import (
	"testing"
)

const (
	TestKernelConstraint = ">= 6.8.0"
)

/* TODO
func TestIsCiliumBinaryExists(t *testing.T) {
	// Test cases for isCiliumBinaryExists
}
*/

/* TODO
func TestGetCiliumVersionByCNI(t *testing.T) {
	// Test cases for getCiliumVersionByCNI
}
*/

func TestCheckCiliumVersion(t *testing.T) {
	testCases := []struct {
		name          string
		inVerStr      string
		expectCondMet bool
		expectError   bool
	}{
		{
			name: "input string is ok, and version is match",
			inVerStr: `Cilium CNI plugin 1.17.4  go version go1.24.6 linux/amd64
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: true,
			expectError:   false,
		},
		{
			name: "input string is ok, and version is match 2",
			inVerStr: `Cilium CNI plugin 1.17.4  go version go1.24.6 linux/amd64
`,
			expectCondMet: true,
			expectError:   false,
		},
		{
			name:          "input string is ok, and version is match 3",
			inVerStr:      `Cilium CNI plugin 1.17.4  go version go1.24.6 linux/amd64`,
			expectCondMet: true,
			expectError:   false,
		},
		{
			name: "input string is ok, and version is match 4",
			inVerStr: `CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0
Cilium CNI plugin 1.17.4  go version go1.24.6 linux/amd64`,
			expectCondMet: true,
			expectError:   false,
		},
		{
			name: "input string is ok, but version is not match",
			inVerStr: `Cilium CNI plugin 1.14.14  go version go1.24.3 linux/amd64
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: false,
			expectError:   true,
		},
		{
			name: "input string is ok but version is not match 2",
			inVerStr: `Cilium CNI plugin 1.14.14-custom-build  go version go1.24.3 linux/amd64
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: false,
			expectError:   true,
		},
		{
			name: "input string is ok but wrong semver format",
			inVerStr: `Cilium CNI plugin 1.14 .14  go version go1.24.3 linux/amd64
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: false,
			expectError:   true,
		},
		{
			name: "input string is bad: No startPhrase",
			inVerStr: `1.17.4  go version go1.24.3 linux/amd64
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: false,
			expectError:   true,
		},
		{
			name: "input string is ok: No endPhrase",
			inVerStr: `Cilium CNI plugin 1.17.4
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: true,
			expectError:   false,
		},
		{
			name: "input string is bad: No endPhrase 2",
			inVerStr: `Cilium CNI plugin 1.17.4 go1.24.3 linux/amd64
CNI protocol versions supported: 0.1.0, 0.2.0, 0.3.0, 0.3.1, 0.4.0, 1.0.0`,
			expectCondMet: false,
			expectError:   true,
		},
		{
			name:          "input string is ok: No endPhrase 3",
			inVerStr:      `Cilium CNI plugin 1.17.4`,
			expectCondMet: true,
			expectError:   false,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			isCiliumAlreadyUpgraded, err := checkCiliumVersion(test.inVerStr, ciliumConstraintDef)

			switch test.expectError {
			case true:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case false:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			switch test.expectCondMet {
			case false:
				if isCiliumAlreadyUpgraded {
					t.Fatalf("expected false but received true")
				} else {
					t.Logf("expected true and received true")
				}
			case true:
				if !isCiliumAlreadyUpgraded {
					t.Fatalf("expected true but received false")
				}
			}
		})
	}
}

/* TODO
func TestCheckWireGuardInterfacesOnNode(t *testing.T) {
	// Test cases for checkWireGuardInterfacesOnNode
}
*/

/* TODO
func TestGetCurrentKernelVersion(t *testing.T) {
	// Test cases for getCurrentKernelVersion
}
*/

func TestCheckKernelVersionWGCiliumRequirements(t *testing.T) {
	testCases := []struct {
		name          string
		inVerStr      string
		expectCondMet bool
		expectError   bool
	}{
		{
			name:          "Version parsing ok and met requirements",
			inVerStr:      "6.8.0",
			expectCondMet: true,
			expectError:   false,
		},
		{
			name:          "Version parsing ok and met requirements",
			inVerStr:      "6.8.4-ogogo",
			expectCondMet: true,
			expectError:   false,
		},
		{
			name:          "Version parsing ok and not met requirements",
			inVerStr:      "5.15.15",
			expectCondMet: false,
			expectError:   false,
		},
		{
			name:          "Version parsing failed",
			inVerStr:      "6.8.4.15",
			expectCondMet: false,
			expectError:   true,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			isKernelVersionMeet, err := checkKernelVersionWGCiliumRequirements(test.inVerStr, TestKernelConstraint)

			switch test.expectError {
			case true:
				if err == nil {
					t.Fatalf("expected error but received none")
				} else {
					t.Logf("expected error and got it: %v", err)
				}
			case false:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			switch test.expectCondMet {
			case false:
				if isKernelVersionMeet {
					t.Fatalf("expected false but received true")
				} else {
					t.Logf("expected false and received false")
				}
			case true:
				if !isKernelVersionMeet {
					t.Fatalf("expected true but received false")
				}
			}
		})
	}
}
