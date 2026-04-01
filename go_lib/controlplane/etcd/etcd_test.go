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

package etcd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

func TestAddMembersToPodManifest(t *testing.T) {
	tests := []struct {
		name           string
		podManifest    string
		initialCluster []*etcdserverpb.Member
		want           string
	}{
		{
			name: "single member",
			podManifest: `
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=node1=https://1.1.1.1:2380
`,
			initialCluster: []*etcdserverpb.Member{
				{Name: "node1", PeerURLs: []string{"https://1.1.1.1:2380"}},
			},
			want: `
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=node1=https://1.1.1.1:2380
`,
		},
		{
			name: "multiple members",
			podManifest: `
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=node1=https://1.1.1.1:2380
`,
			initialCluster: []*etcdserverpb.Member{
				{Name: "node1", PeerURLs: []string{"https://1.1.1.1:2380"}},
				{Name: "node2", PeerURLs: []string{"https://2.2.2.2:2380"}},
			},
			want: `
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=node1=https://1.1.1.1:2380,node2=https://2.2.2.2:2380
`,
		},
		{
			name: "replace existing multi-member list",
			podManifest: `
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=old1=https://1.1.1.1:2380,old2=https://2.2.2.2:2380
`,
			initialCluster: []*etcdserverpb.Member{
				{Name: "node1", PeerURLs: []string{"https://10.0.0.1:2380"}},
				{Name: "node2", PeerURLs: []string{"https://10.0.0.2:2380"}},
			},
			want: `
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=node1=https://10.0.0.1:2380,node2=https://10.0.0.2:2380
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addMembersToPodManifest([]byte(tt.podManifest), t.Name(), tt.initialCluster)
			if string(got) != tt.want {
				t.Errorf("addMembersToPodManifest() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestPrepareAndWriteEtcdStaticPod(t *testing.T) {
	tmpDir := t.TempDir()
	config := prepareOptions(WithManifestDir(tmpDir))

	podManifest := []byte(`
spec:
  containers:
  - command:
    - etcd
    - --initial-cluster=node1=https://1.1.1.1:2380
`)
	nodeName := "node1"
	initialCluster := []*etcdserverpb.Member{
		{Name: "node1", PeerURLs: []string{"https://1.1.1.1:2380"}},
		{Name: "node2", PeerURLs: []string{"https://2.2.2.2:2380"}},
	}

	err := prepareAndWriteEtcdStaticPod(podManifest, config, nodeName, initialCluster)
	if err != nil {
		t.Fatalf("prepareAndWriteEtcdStaticPod() failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, constants.Etcd+".yaml")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected file %s does not exist", expectedFile)
	}

	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	expectedSubstring := "--initial-cluster=node1=https://1.1.1.1:2380,node2=https://2.2.2.2:2380"
	if !bytes.Contains(content, []byte(expectedSubstring)) {
		t.Errorf("written content does not contain expected substring %q\nContent:\n%s", expectedSubstring, string(content))
	}
}
