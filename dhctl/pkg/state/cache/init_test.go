// Copyright 2023 Flant JSC
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

package cache

import "testing"

func TestCacheIdentity(t *testing.T) {
	tests := []struct {
		name              string
		kubeconfigPath    string
		kubeconfigContext string
		want              string
	}{
		{"Cache identity with kubeconfigPath and kubeconfigContext", "foo", "bar", "kubeconfig-c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2"},
		{"Cache identity with kubeconfigPath", "foo", "", "kubeconfig-2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
		{"Cache identity without kubeconfigPath", "", "", ""},
	}
	// The execution loop
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := GetCacheIdentityFromKubeconfig(tt.kubeconfigPath, tt.kubeconfigContext)
			if ans != tt.want {
				t.Errorf("got %s, want %s", ans, tt.want)
			}
		})
	}
}
