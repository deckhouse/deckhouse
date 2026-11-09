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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// NotWritableError says that the registry cannot write to its store, and what to do about it.
type NotWritableError struct {
	// Dir is the directory a file could not be created in, as the container sees it, and HostDir the
	// same directory on the node — empty when the store's directory on the node is not known.
	Dir, HostDir string

	// HostPath is the store's directory on the node, which is what to change the owner or the mode
	// of. Empty when it is not known.
	HostPath string

	// Err is what creating the file failed with.
	Err error

	// Owner and Mode describe Dir, when it could be read.
	Owner, Group uint32
	Mode         fs.FileMode
	Described    bool
}

func (e *NotWritableError) Error() string {
	where := e.Dir
	if e.HostDir != "" {
		where = fmt.Sprintf("%s (%s on the node)", e.Dir, e.HostDir)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "the store is not writable: creating a file in %s failed: %v", where, cause(e.Err))
	if e.Described {
		fmt.Fprintf(&text, "; the directory belongs to uid %d, gid %d, mode %s", e.Owner, e.Group, e.Mode)
	}

	if errors.Is(e.Err, syscall.EROFS) {
		text.WriteString("; the filesystem under the store is mounted read-only, and has to be " +
			"mounted read-write")
		return text.String()
	}

	target := e.HostPath
	if target == "" {
		target = "the store's directory on the node"
	}
	dir := e.HostDir
	if dir == "" {
		dir = e.Dir
	}

	// Ownership and mode bits answer EACCES and never EPERM: a directory that refuses root with EPERM
	// carries an attribute no owner changes, or a security module stands in the way.
	if errors.Is(e.Err, syscall.EPERM) {
		fmt.Fprintf(&text, "; neither owner nor mode refuse a write with this error: the directory "+
			"probably carries the immutable attribute — lsattr -d %s, and chattr -i to clear it — or a "+
			"security module such as SELinux or AppArmor denies it", dir)
		return text.String()
	}

	text.WriteString("; the registry runs as root without capabilities, so it can only write where " +
		"root owns the directory and its mode lets the owner write: ")
	chown := !e.Described || e.Owner != 0
	chmod := !e.Described || e.Mode.Perm()&0o300 != 0o300
	switch {
	case chown && chmod:
		fmt.Fprintf(&text, "chown -R root:root %[1]s && chmod -R u+rwX %[1]s", target)
	case chown:
		fmt.Fprintf(&text, "chown -R root:root %s", target)
	case chmod:
		fmt.Fprintf(&text, "chmod -R u+rwX %s", target)
	default:
		fmt.Fprintf(&text, "this one does both, so an access control list or a security module such "+
			"as SELinux or AppArmor denies it — getfacl -p %s", dir)
	}
	return text.String()
}

// cause is the error without the operation and the path os puts around it: the message names the
// directory itself.
func cause(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// CheckWritable creates a file in each directory of the store the registry writes into and removes
// it, and reports a *NotWritableError when the store refuses it for its permissions: EACCES, EPERM or
// EROFS. hostPath is the store's directory on the node, for the advice in the message.
//
// Because the registry does not say so itself. It runs as root but drops every capability, and root
// without CAP_DAC_OVERRIDE is held to a directory's mode bits like anyone else: a store directory an
// operator created, or chowned, to another owner reads fine and refuses every write. The registry
// starts, serves what is there, and answers each push and each cache miss it would keep with a 500
// that names no file — and the replica reports an error about the copy, never about the directory.
//
// The directories the registry creates its files in, of those that exist: the blobs and the
// repositories, and the tree above them — docker/registry/v2 once it is there, and before that the
// deepest of the store and the directories down to it, which is where the registry creates the rest
// of the tree. Not the store itself once the tree is there: the registry writes nothing beside it, and
// a store directory chowned to another user over a root-owned tree is one it fills without a fault —
// reporting it would turn a store that works into one that refuses writes, with an alert to say so.
// Other failures are not this check's to report — a full disk is the reserve's, and a missing
// directory is one the registry has not created yet.
func CheckWritable(dataDir, hostPath string) error {
	for _, dir := range writtenDirs(dataDir) {
		err := probeWrite(dir)
		if err == nil {
			continue
		}
		if !errors.Is(err, syscall.EACCES) && !errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.EROFS) {
			continue
		}

		refused := &NotWritableError{Dir: dir, HostPath: hostPath, Err: err}
		if hostPath != "" {
			if relative, err := filepath.Rel(dataDir, dir); err == nil {
				refused.HostDir = filepath.Join(hostPath, relative)
			}
		}
		if info, err := os.Stat(dir); err == nil {
			refused.Mode = info.Mode()
			if owner, ok := info.Sys().(*syscall.Stat_t); ok {
				refused.Owner, refused.Group, refused.Described = owner.Uid, owner.Gid, true
			}
		}
		return refused
	}
	return nil
}

// writtenDirs is what CheckWritable probes, of the directories that exist — see there.
func writtenDirs(dataDir string) []string {
	isDir := func(dir string) bool {
		info, err := os.Stat(dir)
		return err == nil && info.IsDir()
	}
	if !isDir(dataDir) {
		return nil
	}

	// A directory that cannot be read counts as absent, so that one the registry cannot even pass
	// through is reported where it stands: the deepest that could be read is the one above it.
	deepest := dataDir
	for _, dir := range []string{"docker", "registry", "v2"} {
		next := filepath.Join(deepest, dir)
		if !isDir(next) {
			return []string{deepest}
		}
		deepest = next
	}
	dirs := []string{deepest}
	for _, dir := range []string{"blobs", "repositories"} {
		if next := filepath.Join(deepest, dir); isDir(next) {
			dirs = append(dirs, next)
		}
	}
	return dirs
}

// probeFile is the name the probe creates its file under: the same every time, so that a directory
// that takes the file and refuses to remove it keeps one, and not one for every probe.
const probeFile = ".write-probe"

// probeWrite creates an empty file in dir and removes it. A variable so that the tests can refuse it:
// they may run as root with every capability, which no mode bit stops.
//
// The creation is what is probed, and the removal only tidies up. An append-only directory takes new
// files and refuses with EPERM to remove them, which the registry never asks of the directories probed
// here — it creates its tree in them and writes below it — so a removal refused that way is not a
// refusal to write, and the file stays. The next probe finds it, cannot remove it either, and passes on
// what the file shows: the directory took it. A directory made immutable after it was append-only
// passes too, and its refusal comes from the registry's own writes.
//
// The loop and the store watcher may probe at once and meet on the name. What one of them gets then —
// the file already there, or gone before it removes it — is not a refusal, and CheckWritable passes
// over it.
var probeWrite = func(dir string) error {
	name := filepath.Join(dir, probeFile)
	switch err := removeFile(name); {
	case err == nil, errors.Is(err, fs.ErrNotExist):
	case errors.Is(err, syscall.EPERM):
		return nil
	default:
		return err
	}

	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		_ = removeFile(name)
		return err
	}
	if err := removeFile(name); err != nil && !errors.Is(err, syscall.EPERM) {
		return err
	}
	return nil
}

// removeFile is os.Remove, or what a test puts in its place: an append-only directory takes chattr and
// a filesystem that keeps the attribute, which a test run has neither of.
var removeFile = os.Remove
