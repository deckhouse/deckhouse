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

package project

import "testing"

func TestDir(t *testing.T) {
	tests := []struct {
		pwd     string
		want    string
		wantErr bool
	}{
		{
			pwd:  "/deckhouse",
			want: "/deckhouse",
		},
		{
			pwd:  "/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release",
			want: "/deckhouse",
		},
		{
			pwd:  "/home/user/work/flant/deckhouse",
			want: "/home/user/work/flant/deckhouse",
		},
		{
			pwd:  "/home/user/work/flant/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release",
			want: "/home/user/work/flant/deckhouse",
		},
		{
			pwd:  "/home/user/src/github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release",
			want: "/home/user/src/github.com/deckhouse/deckhouse",
		},
		{
			pwd:  "/home/user/src/github.com/deckhouse",
			want: "/home/user/src/github.com/deckhouse",
		},
		{
			pwd:  "/home/user/src",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.pwd, func(t *testing.T) {
			got, err := dir(tt.pwd)
			if (err != nil) != tt.wantErr {
				t.Errorf("dir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("dir() got = %v, want %v", got, tt.want)
			}
		})
	}
}
