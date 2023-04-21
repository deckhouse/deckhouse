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

package main

import (
	"encoding/json"
	"testing"
)

const kubeletConfigJSON = `
{
  "kubeletconfig": {
    "evictionHard": {
      "imagefs.available": "5%",
      "imagefs.inodesFree": "5%",
      "memory.available": "1%",
      "nodefs.available": "5%",
      "nodefs.inodesFree": "5%"
    },
    "evictionSoft": {
      "imagefs.available": "10%",
      "imagefs.inodesFree": "10%",
      "memory.available": "2%",
      "nodefs.available": "10%",
      "nodefs.inodesFree": "10%"
    },
    "evictionSoftGracePeriod": {
      "imagefs.available": "1m30s",
      "imagefs.inodesFree": "1m30s",
      "memory.available": "1m30s",
      "nodefs.available": "1m30s",
      "nodefs.inodesFree": "1m30s"
    }
  }
}
`

func Test_KubeletConfig(t *testing.T) {
	var kubeletConfig *KubeletConfig

	err := json.Unmarshal([]byte(kubeletConfigJSON), &kubeletConfig)
	if err != nil {
		t.Fatal(err)
	}

	if len(kubeletConfig.KubeletConfiguration.EvictionHard) == 0 || len(kubeletConfig.KubeletConfiguration.EvictionSoft) == 0 {
		t.Fatal("EvictionSoft and EvictionHard are empty")
	}
}

func Test_extractPercent(t *testing.T) {
	_ = map[string]string{
		"imagefs.available":  "10%",
		"imagefs.inodesFree": "10%",
		"nodefs.available":   "500Mi",
		"nodefs.inodesFree":  "500",
	}

	type args struct {
		realResource uint64
		signal       string
		evictionMap  map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"ten percent",
			args{
				realResource: 4000,
				signal:       "imagefs.available",
				evictionMap: map[string]string{
					"imagefs.available": "10%",
				},
			},
			"10",
			false,
		},
		{
			"bytes",
			args{
				realResource: 107374182400, // 100 GiB
				signal:       "imagefs.available",
				evictionMap: map[string]string{
					"imagefs.available": "10Gi",
				},
			},
			"10.00",
			false,
		},
		{
			"inodes",
			args{
				realResource: 100,
				signal:       "imagefs.inodes",
				evictionMap: map[string]string{
					"imagefs.inodes": "10",
				},
			},
			"10.00",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPercent(tt.args.realResource, tt.args.signal, tt.args.evictionMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPercent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractPercent() got = %v, want %v", got, tt.want)
			}
		})
	}
}
