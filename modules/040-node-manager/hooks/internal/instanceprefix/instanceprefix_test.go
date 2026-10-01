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

package instanceprefix

import (
	"testing"

	"github.com/tidwall/gjson"
)

type jsonValues string

func (v jsonValues) Get(path string) gjson.Result {
	return gjson.Get(string(v), path)
}

func (v jsonValues) GetOk(path string) (gjson.Result, bool) {
	res := gjson.Get(string(v), path)
	return res, res.Exists()
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		values string
		want   string
	}{
		{
			name:   "nothing set",
			values: `{"global": {}, "nodeManager": {}}`,
			want:   "",
		},
		{
			name:   "deprecated ClusterConfiguration.cloud.prefix only",
			values: `{"global": {"clusterConfiguration": {"cloud": {"prefix": "cloud"}}}}`,
			want:   "cloud",
		},
		{
			name:   "global.prefix wins over ClusterConfiguration.cloud.prefix",
			values: `{"global": {"prefix": "global", "clusterConfiguration": {"cloud": {"prefix": "cloud"}}}}`,
			want:   "global",
		},
		{
			name:   "nodeManager.instancePrefix wins over global.prefix",
			values: `{"global": {"prefix": "global"}, "nodeManager": {"instancePrefix": "kube"}}`,
			want:   "kube",
		},
		{
			name:   "nodeManager.internal.instancePrefix is never read",
			values: `{"global": {"prefix": "global"}, "nodeManager": {"internal": {"instancePrefix": "stale"}}}`,
			want:   "global",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(jsonValues(tt.values)); got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}
