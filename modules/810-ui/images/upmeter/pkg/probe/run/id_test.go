/*
Copyright 2023 Flant JSC

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

package run

import (
	"testing"
)

func Test_AgentUniqueId(t *testing.T) {
	a, b := ID(), ID()
	if a != b {
		t.Errorf("expected %q == %q", a, b)
	}
}

func Test_RandomIdentifier(t *testing.T) {
	prefix := "upmeter-test-object"
	a, b := RandomIdentifier(prefix), RandomIdentifier(prefix)
	if a == b {
		t.Errorf("expected %q != %q", a, b)
	}
}

func Test_nodeNameHash(t *testing.T) {
	tests := []struct {
		nodeName string
		want     string
	}{
		{nodeName: "", want: "35d78cbb"},
		{nodeName: "kube-master", want: "36174012"},
		{nodeName: "kube-master-0", want: "c349c19b"},
		{nodeName: "kube-master-1", want: "57fb8ddf"},
		{nodeName: "kube-master-2", want: "2a547f6c"},
		{nodeName: "dev2-master-0", want: "7131bf4e"},
		{nodeName: "dev2-master-1", want: "ccbabc3"},
		{nodeName: "dev2-master-2", want: "35dc4115"},
	}
	for _, tt := range tests {
		t.Run(tt.nodeName, func(t *testing.T) {
			if got := nodeNameHash(tt.nodeName); got != tt.want {
				t.Errorf("nodeNameHash(%q) = %v, want %v", tt.nodeName, got, tt.want)
			}
		})
	}
}
