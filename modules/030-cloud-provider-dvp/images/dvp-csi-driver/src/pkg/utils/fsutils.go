/*
Copyright 2025 Flant JSC

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

package utils

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func StatFS(path string) (int64, int64, int64, int64, int64, int64, error) {
	var (
		available  int64
		capacity   int64
		used       int64
		inodesFree int64
		inodes     int64
		inodesUsed int64
	)

	statfs := &unix.Statfs_t{}
	if err := unix.Statfs(path, statfs); err != nil {
		err = fmt.Errorf("failed to get fs info on path %s: %v", path, err)
		return available, capacity, used, inodesFree, inodes, inodesUsed, err
	}

	// Available is blocks available * fragment size
	available = int64(statfs.Bavail) * int64(statfs.Bsize) // nolint:unconvert

	// Capacity is total block count * fragment size
	capacity = int64(statfs.Blocks) * int64(statfs.Bsize) // nolint:unconvert

	// Usage is block being used * fragment size (aka block size).
	used = (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize) // nolint:unconvert

	// Get inode usage
	inodes = int64(statfs.Files)
	inodesFree = int64(statfs.Ffree)
	inodesUsed = inodes - inodesFree

	return available, capacity, used, inodesFree, inodes, inodesUsed, nil
}

func IsBlockDevice(fullPath string) (bool, error) {
	st := &unix.Stat_t{}
	err := unix.Stat(fullPath, st)
	if err != nil {
		return false, err
	}

	return (st.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

func GetBlockSizeBytes(devicePath string) (int64, error) {
	cmd := exec.Command("blockdev", "--getsize64", devicePath)
	out, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", devicePath, string(out), err)
	}

	strOut := strings.TrimSpace(string(out))
	gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse %s into an int size", strOut)
	}

	return gotSizeBytes, nil
}

func GetDeviceByMountPoint(mp string) (string, error) {
	out, err := exec.Command("findmnt", "-nfc", mp).Output()
	if err != nil {
		return "", fmt.Errorf("error: %w", err)
	}

	s := strings.Fields(string(out))
	if len(s) < 2 {
		return "", fmt.Errorf("could not parse command output: >%s<", string(out))
	}
	return s[1], nil
}
