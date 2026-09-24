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

package app

import (
	"strings"
	"testing"
)

func TestValidateNodeName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
		why   string
	}{
		{"", true, "an unset name leaves the node named after its hostname"},
		{"master-0", true, "the ordinary case"},
		{"master-0.example.com", true, "an FQDN is a DNS subdomain and kubelet registers under it"},
		{"m", true, "a single character is a label"},
		{"Master-0", false, "kubelet lowercases what it is given, so an uppercase name would never match"},
		{"master_0", false, "an underscore is not allowed in a DNS label"},
		{"-master", false, "a label cannot start with a dash"},
		{"master-", false, "a label cannot end with a dash"},
		{"master..0", false, "an empty label"},
		{"master 0", false, "a space"},
		{strings.Repeat("a", 253), true, "253 characters is the limit, not past it"},
		{strings.Repeat("a", 254), false, "past the 253 characters an object name allows"},
	}

	for _, tc := range cases {
		err := ValidateNodeName(tc.name)
		if tc.valid && err != nil {
			t.Errorf("%q should be accepted (%s), got: %v", tc.name, tc.why, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("%q should be rejected (%s)", tc.name, tc.why)
		}
	}
}
