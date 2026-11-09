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
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

// MountInfo is where the kernel lists the mounts this process sees.
const MountInfo = "/proc/self/mountinfo"

// StoreDirectory is the name of the store's directory on the node: the storage pod mounts
// `<storePath>/local_data`, and the storePath is where a disk for the store is meant to be mounted.
const StoreDirectory = "local_data"

// Filesystem is what the syncer found out about the filesystem under the store.
type Filesystem struct {
	// Dedicated is true when the store has a filesystem of its own, and false when it shares one with
	// the node — or when that could not be told, which is the same answer on purpose. See
	// DetectFilesystem.
	Dedicated bool

	// Why says what decided it, in a sentence for the log.
	Why string

	// Root is the directory of its filesystem the store's mount shows, as the kernel names it: "/"
	// for a disk mounted at the store's directory, "/local_data" for one mounted at the store path
	// above it, and the whole path from the node's root for a directory of the node's own filesystem.
	// Empty when the store is not a mount at all.
	Root string

	// Type is the filesystem's type, and Device its major:minor, as the kernel names them.
	Type   string
	Device string
}

// referenceMounts are files the kubelet and containerd put into every container from their own
// directories, so that their mounts name the filesystems those two live on: the kubelet's root
// directory, which is what its eviction thresholds for the node filesystem are measured on, and
// containerd's, which is what its thresholds for the image filesystem are measured on.
var referenceMounts = []string{
	// The kubelet's pod directory.
	"/dev/termination-log",
	"/etc/hosts",
	// containerd's sandbox directory.
	"/etc/hostname",
	"/etc/resolv.conf",
}

// dedicatedTypes are the filesystems whose device numbers say whether two mounts share their space.
//
// Others are taken as shared whatever the numbers say. A btrfs subvolume and a zfs dataset each
// carry a number of their own while drawing on the same pool as the filesystem beside them, which
// is exactly the sharing the reserve is about; and anything more exotic is not what a disk for the
// store is formatted with.
//
// A thin LVM volume beside a thin root volume of the same pool is taken as dedicated, and rightly so
// for what the reserve guards: the kubelet measures its eviction thresholds on the root filesystem's
// own figures, which do not move as the store's volume fills, so the kubelet's share kept on the
// store would not keep the kubelet from evicting, nor the pool from running out — an overcommitted
// pool is guarded by the pool's own monitoring and autoextension, whatever either filesystem keeps.
var dedicatedTypes = map[string]bool{"ext2": true, "ext3": true, "ext4": true, "xfs": true}

// DetectFilesystem tells whether the store at dataDir has a filesystem of its own, from the mounts
// listed in mountInfo. storeDirectory is the name of the store's directory on the node.
//
// From the mount table rather than from comparing stat(2) device numbers, which differ between
// btrfs subvolumes of one filesystem, and from statfs figures, which move between two calls on a
// filesystem being written. The mount table names the filesystem itself, and — in its root field —
// which directory of it the store is.
//
// Dedicated only when everything says so: the store's mount is a disk mounted at the store's
// directory or at the store path above it; the filesystem is one whose device number can be
// trusted; and neither the kubelet's nor containerd's files come from the same device. Any doubt —
// a mount table that cannot be read, nothing to compare with, a store path mounted from higher up —
// is shared. Mistaking a shared filesystem for a dedicated one leaves the reserve below the kubelet's
// eviction threshold; the other mistake only keeps a few more gigabytes free.
func DetectFilesystem(mountInfo, dataDir, storeDirectory string) Filesystem {
	content, err := os.ReadFile(mountInfo)
	if err != nil {
		return Filesystem{Why: fmt.Sprintf("the mounts cannot be read, so the filesystem is taken as shared: %v", err)}
	}
	return detectFilesystem(string(content), dataDir, storeDirectory)
}

func detectFilesystem(mountInfo, dataDir, storeDirectory string) Filesystem {
	mounts := parseMountInfo(mountInfo)

	store, found := lastMount(mounts, path.Clean(dataDir))
	if !found {
		return Filesystem{Why: fmt.Sprintf("%s is not a mount of its own, so its filesystem is the "+
			"container's and taken as shared", dataDir)}
	}
	result := Filesystem{Root: store.root, Type: store.fstype, Device: store.device}

	if !dedicatedTypes[store.fstype] {
		result.Why = fmt.Sprintf("the store is on %s, whose space may be shared with filesystems "+
			"beside it, so it is taken as shared", store.fstype)
		return result
	}

	if store.root != "/" && store.root != "/"+storeDirectory {
		result.Why = fmt.Sprintf("the store is the directory %s of a larger filesystem, not a disk "+
			"mounted for it", store.root)
		return result
	}

	compared := 0
	for _, point := range referenceMounts {
		reference, found := lastMount(mounts, point)
		if !found {
			continue
		}
		compared++
		if reference.device == store.device {
			result.Why = fmt.Sprintf("the store is on device %s, which also holds %s, so it shares "+
				"its filesystem with the kubelet or containerd", store.device, point)
			return result
		}
	}
	if compared == 0 {
		result.Why = "no mount of the kubelet's or containerd's to compare the store's filesystem " +
			"with, so it is taken as shared"
		return result
	}

	result.Dedicated = true
	result.Why = fmt.Sprintf("the store is a %s filesystem of its own on device %s, mounted at %s",
		store.fstype, store.device, mountedAt(store.root))
	return result
}

// mountedAt says where on the node the disk a dedicated store is on is mounted, from its root field.
func mountedAt(root string) string {
	if root == "/" {
		return "the store's directory"
	}
	return "the store path above the store's directory"
}

// mount is one line of a mount table: the device, the directory of the filesystem the mount shows,
// where it is mounted, and the filesystem's type.
type mount struct {
	device, root, point, fstype string
}

// parseMountInfo reads the mount table in the format of proc(5)'s mountinfo:
//
//	36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
//
// The optional fields before the separator vary in number, so the type is found after it rather
// than at a fixed position. A line that does not parse is skipped: what it would have said is then
// not known, and every unknown counts toward shared.
func parseMountInfo(content string) []mount {
	var mounts []mount
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		separator := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				separator = i
				break
			}
		}
		if separator < 0 || separator+1 >= len(fields) {
			continue
		}
		mounts = append(mounts, mount{
			device: fields[2],
			root:   unescapeMount(fields[3]),
			point:  unescapeMount(fields[4]),
			fstype: fields[separator+1],
		})
	}
	return mounts
}

// lastMount is the mount at point that is visible: the last one listed, since a later mount over the
// same point hides the ones before it.
func lastMount(mounts []mount, point string) (mount, bool) {
	for i := len(mounts) - 1; i >= 0; i-- {
		if mounts[i].point == point {
			return mounts[i], true
		}
	}
	return mount{}, false
}

// unescapeMount undoes the octal escapes the kernel writes for a space, a tab, a newline and a
// backslash in a path.
func unescapeMount(field string) string {
	if !strings.Contains(field, `\`) {
		return field
	}
	var out strings.Builder
	for i := 0; i < len(field); i++ {
		if field[i] == '\\' && i+4 <= len(field) {
			if value, err := strconv.ParseUint(field[i+1:i+4], 8, 8); err == nil {
				out.WriteByte(byte(value))
				i += 3
				continue
			}
		}
		out.WriteByte(field[i])
	}
	return out.String()
}
