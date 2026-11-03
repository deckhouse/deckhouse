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

package guard

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

const mptcpSysctl = "/proc/sys/net/mptcp/enabled"

type NetNS struct {
	Name string
	f    *os.File
}

// errNotNetNS: the path exists but no netns is mounted on it, e.g. a file containerd left
// after a failed sandbox teardown or has not bind-mounted yet.
var errNotNetNS = errors.New("not a mounted network namespace")

func OpenNetNS(name, path string) (*NetNS, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &fs); err != nil {
		f.Close()
		return nil, fmt.Errorf("statfs %s: %w", path, err)
	}
	if fs.Type != unix.NSFS_MAGIC {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, errNotNetNS)
	}
	return &NetNS{Name: name, f: f}, nil
}

func (n *NetNS) Fd() int { return int(n.f.Fd()) }

func (n *NetNS) Close() error { return n.f.Close() }

// Do runs fn on a locked OS thread switched into the netns; children exec'd from fn inherit it.
func (n *NetNS) Do(fn func() error) error {
	errCh := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		// The thread is not unlocked: if switching back fails it must not be reused.
		orig, err := os.Open("/proc/thread-self/ns/net")
		if err != nil {
			errCh <- err
			return
		}
		defer orig.Close()
		if err := unix.Setns(n.Fd(), unix.CLONE_NEWNET); err != nil {
			errCh <- fmt.Errorf("setns %s: %w", n.Name, err)
			return
		}
		ferr := fn()
		if err := unix.Setns(int(orig.Fd()), unix.CLONE_NEWNET); err != nil {
			errCh <- errors.Join(ferr, fmt.Errorf("setns back: %w", err))
			return
		}
		runtime.UnlockOSThread()
		errCh <- ferr
	}()
	return <-errCh
}

// MPTCP returns net.mptcp.enabled in the netns, or mptcpAbsent.
func (n *NetNS) MPTCP() (string, error) {
	var v string
	err := n.Do(func() error {
		b, err := os.ReadFile(mptcpSysctl)
		if errors.Is(err, os.ErrNotExist) {
			v = mptcpAbsent
			return nil
		}
		v = strings.TrimSpace(string(b))
		return err
	})
	return v, err
}

func (n *NetNS) SetMPTCP(v string) error {
	return n.Do(func() error { return os.WriteFile(mptcpSysctl, []byte(v), 0o644) })
}
