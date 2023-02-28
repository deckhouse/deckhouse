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

package context

import "testing"

func Test_buildUri(t *testing.T) {
	type args struct {
		path string
		opts map[string]string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "absolute path",
			args: args{path: "/db"},
			want: "/db",
		},
		{
			name: "relative path",
			args: args{path: "./db"},
			want: "./db",
		},
		{
			name: "file-relative path",
			args: args{path: "file://./db"},
			want: "file://./db",
		},
		{
			name: "path with options",
			args: args{
				path: "/db",
				opts: map[string]string{"a": "aaa", "x": "xxx"},
			},
			want: "/db?a=aaa&x=xxx",
		},
		{
			name: ":memory:",
			args: args{path: ":memory:"},
			want: ":memory:",
		},
		{
			name: ":memory: with options",
			args: args{
				path: ":memory:",
				opts: map[string]string{"a": "aaa", "x": "xxx"},
			},
			want: ":memory:?a=aaa&x=xxx",
		},
	}

	for _, tt := range tests {
		got := buildUri(tt.args.path, tt.args.opts)
		if got != tt.want {
			t.Errorf("uriby(): got=%s, want=%s", got, tt.want)
		}
	}
}
