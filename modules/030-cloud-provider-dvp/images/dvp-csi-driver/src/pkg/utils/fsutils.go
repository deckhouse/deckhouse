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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

func StatFS(path string) (available, capacity, used, inodesFree, inodes, inodesUsed int64, err error) {
	statfs := &unix.Statfs_t{}
	err = unix.Statfs(path, statfs)
	if err != nil {
		err = fmt.Errorf("failed to get fs info on path %s: %v", path, err)
		return
	}

	// Available is blocks available * fragment size
	available = int64(statfs.Bavail) * int64(statfs.Bsize)

	// Capacity is total block count * fragment size
	capacity = int64(statfs.Blocks) * int64(statfs.Bsize)

	// Usage is block being used * fragment size (aka block size).
	used = (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize)

	// Get inode usage
	inodes = int64(statfs.Files)
	inodesFree = int64(statfs.Ffree)
	inodesUsed = inodes - inodesFree

	return
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

// getDeviceInfo will return the first Device which is a partition and its filesystem.
// if the given Device disk has no partition then an empty zero valued device will return
func GetDeviceInfo(device string) (string, error) {
	devicePath, err := filepath.EvalSymlinks(device)
	if err != nil {
		klog.Errorf("Unable to evaluate symlink for device %s", device)
		return "", errors.New(err.Error())
	}

	klog.Info("lsblk -nro FSTYPE ", devicePath)
	cmd := exec.Command("lsblk", "-nro", "FSTYPE", devicePath)
	out, err := cmd.Output()
	exitError, incompleteCmd := err.(*exec.ExitError)
	if err != nil && incompleteCmd {
		return "", errors.New(err.Error() + "lsblk failed with " + string(exitError.Stderr))
	}

	reader := bufio.NewReader(bytes.NewReader(out))
	line, _, err := reader.ReadLine()
	if err != nil {
		klog.Errorf("Error occured while trying to read lsblk output")
		return "", err
	}
	return string(line), nil
}

func MakeFS(device string, fsType string) error {
	// caution, use force flag when creating the filesystem if it doesn't exit.
	klog.Infof("Mounting device %s, with FS %s", device, fsType)

	var cmd *exec.Cmd
	var stdout, stderr bytes.Buffer
	if strings.HasPrefix(fsType, "ext") {
		cmd = exec.Command("mkfs", "-F", "-t", fsType, device)
	} else if strings.HasPrefix(fsType, "xfs") {
		cmd = exec.Command("mkfs", "-t", fsType, "-f", device)
	} else {
		return errors.New(fsType + " is not supported, only xfs and ext are supported")
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitError, incompleteCmd := err.(*exec.ExitError)
	if err != nil && incompleteCmd {
		klog.Errorf("stdout: %s", stdout.String())
		klog.Errorf("stderr: %s", stderr.String())
		return errors.New(err.Error() + " mkfs failed with " + exitError.Error())
	}

	return nil
}
