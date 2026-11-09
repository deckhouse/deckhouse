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

package distribution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mounts of a storage pod's syncer container, cut down to the lines the detector reads, with the
// store's line left out to be added per case. The kubelet's and containerd's files come from the
// node's root filesystem on 8:1.
const podMounts = `2265 2200 0:310 / / rw,relatime master:900 - overlay overlay rw,lowerdir=/var/lib/containerd/x,upperdir=/var/lib/containerd/y
2266 2265 0:312 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
2280 2265 8:1 /var/lib/kubelet/pods/0f3a/etc-hosts /etc/hosts rw,relatime - ext4 /dev/sda1 rw
2281 2265 8:1 /var/lib/kubelet/pods/0f3a/containers/syncer/5c1e /dev/termination-log rw,relatime - ext4 /dev/sda1 rw
2282 2265 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/77aa/hostname /etc/hostname rw,relatime - ext4 /dev/sda1 rw
2283 2265 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/77aa/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/sda1 rw
2284 2265 8:1 /var/lib/kubelet/pods/0f3a/volumes/kubernetes.io~empty-dir/config /config rw,relatime - ext4 /dev/sda1 rw
2285 2265 0:300 / /var/run/secrets/kubernetes.io/serviceaccount ro,relatime - tmpfs tmpfs rw,size=65536k
`

func withStore(line string) string {
	return podMounts + line + "\n"
}

func TestTheStoresFilesystemIsToldFromItsMount(t *testing.T) {
	cases := map[string]struct {
		mounts    string
		dedicated bool
		why       string
	}{
		"a disk mounted at the store path, the directory above the store": {
			mounts:    withStore("2290 2265 253:1 /local_data /data rw,relatime - ext4 /dev/mapper/vg-registry rw"),
			dedicated: true,
		},
		"a disk mounted at the store's directory itself": {
			mounts:    withStore("2290 2265 253:1 / /data rw,relatime - xfs /dev/vdb rw"),
			dedicated: true,
		},
		"the node's root filesystem": {
			mounts: withStore("2290 2265 8:1 /opt/deckhouse/registry/local_data /data rw,relatime - ext4 /dev/sda1 rw"),
			why:    "directory /opt/deckhouse/registry/local_data of a larger filesystem",
		},
		"a disk mounted higher up, at /opt": {
			// Not the node's root filesystem, but not one meant for the store either: whatever else
			// lives under /opt writes there too.
			mounts: withStore("2290 2265 253:2 /deckhouse/registry/local_data /data rw,relatime - ext4 /dev/vdc rw"),
			why:    "of a larger filesystem",
		},
		"a disk at the store's directory that also holds the kubelet's directory": {
			mounts: withStore("2290 2265 8:1 / /data rw,relatime - ext4 /dev/sda1 rw"),
			why:    "which also holds /dev/termination-log",
		},
		"a disk that holds containerd's directory and not the kubelet's": {
			mounts: strings.Replace(withStore("2290 2265 253:1 / /data rw,relatime - xfs /dev/vdb rw"),
				"8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/77aa/hostname",
				"253:1 /io.containerd.grpc.v1.cri/sandboxes/77aa/hostname", 1),
			why: "which also holds /etc/hostname",
		},
		"a btrfs subvolume, whose number is its own and whose space is the pool's": {
			mounts: withStore("2290 2265 0:45 /@registry /data rw,relatime - btrfs /dev/sda2 rw,subvol=/@registry"),
			why:    "the store is on btrfs",
		},
		"a zfs dataset mounted at the store's directory": {
			mounts: withStore("2290 2265 0:52 / /data rw,relatime - zfs rpool/registry rw"),
			why:    "the store is on zfs",
		},
		"no mount at the store at all": {
			mounts: podMounts,
			why:    "is not a mount of its own",
		},
		"nothing of the kubelet's or containerd's to compare with": {
			mounts: "2290 2265 253:1 / /data rw,relatime - ext4 /dev/vdb rw\n",
			why:    "no mount of the kubelet's or containerd's",
		},
		"the last of two mounts at the store, which hides the first": {
			mounts: withStore("2290 2265 253:1 / /data rw,relatime - xfs /dev/vdb rw\n" +
				"2291 2265 8:1 /opt/deckhouse/registry/local_data /data rw,relatime - ext4 /dev/sda1 rw"),
			why: "of a larger filesystem",
		},
		"optional fields before the separator": {
			mounts:    withStore("2290 2265 253:1 /local_data /data rw,relatime shared:12 master:3 - ext4 /dev/vdb rw"),
			dedicated: true,
		},
		"a line cut short is skipped, not misread": {
			mounts: withStore("2290 2265 253:1 / /data rw,relatime -"),
			why:    "is not a mount of its own",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			found := detectFilesystem(c.mounts, "/data", StoreDirectory)
			assert.Equal(t, c.dedicated, found.Dedicated, found.Why)
			assert.NotEmpty(t, found.Why)
			if c.why != "" {
				assert.Contains(t, found.Why, c.why)
			}
		})
	}
}

func TestADedicatedStoreSaysWhatItIsOn(t *testing.T) {
	found := detectFilesystem(withStore("2290 2265 253:1 /local_data /data rw,relatime - ext4 /dev/vdb rw"),
		"/data", StoreDirectory)
	require.True(t, found.Dedicated)
	assert.Equal(t, "/local_data", found.Root)
	assert.Equal(t, "ext4", found.Type)
	assert.Equal(t, "253:1", found.Device)
	assert.Contains(t, found.Why, "the store path above")
}

// TestTheStoresDirectoryNameComesFromTheNode: a store path whose last directory is not local_data is
// still told by its own name.
func TestTheStoresDirectoryNameComesFromTheNode(t *testing.T) {
	mounts := withStore("2290 2265 253:1 /store /data rw,relatime - ext4 /dev/vdb rw")
	assert.False(t, detectFilesystem(mounts, "/data", StoreDirectory).Dedicated)
	assert.True(t, detectFilesystem(mounts, "/data", "store").Dedicated)
}

func TestMountPathsAreUnescaped(t *testing.T) {
	mounts := parseMountInfo(`2290 2265 253:1 /a\040b /mnt/with\040space\134x rw - ext4 /dev/vdb rw`)
	require.Len(t, mounts, 1)
	assert.Equal(t, "/a b", mounts[0].root)
	assert.Equal(t, `/mnt/with space\x`, mounts[0].point)
	assert.Equal(t, `\99`, unescapeMount(`\99`), "not an escape: left as it is")
}

func TestAMountTableThatCannotBeReadIsShared(t *testing.T) {
	found := DetectFilesystem(filepath.Join(t.TempDir(), "absent"), "/data", StoreDirectory)
	assert.False(t, found.Dedicated)
	assert.Contains(t, found.Why, "cannot be read")

	path := filepath.Join(t.TempDir(), "mountinfo")
	require.NoError(t, os.WriteFile(path,
		[]byte(withStore("2290 2265 253:1 / /data rw,relatime - xfs /dev/vdb rw")), 0o600))
	assert.True(t, DetectFilesystem(path, "/data", StoreDirectory).Dedicated)
}
