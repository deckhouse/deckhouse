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

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	loopbackInterfaceName = "lo"
	validationChrootDir   = "/validation-chroot"
)

func setupIsolatedProcessChild() error {
	// Temporarily disabled for runtime verification. Restore if loopback-dependent
	// validation paths fail inside the private network namespace.
	// if err := bringLoopbackUp(); err != nil {
	// 	return err
	// }

	if err := enterValidationChroot(); err != nil {
		return err
	}

	return nil
}

func bringLoopbackUp() error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return fmt.Errorf("open control socket: %w", err)
	}
	defer unix.Close(fd)

	req, err := unix.NewIfreq(loopbackInterfaceName)
	if err != nil {
		return fmt.Errorf("build ifreq for %s: %w", loopbackInterfaceName, err)
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, req); err != nil {
		return fmt.Errorf("get %s flags: %w", loopbackInterfaceName, err)
	}
	if req.Uint16()&unix.IFF_UP != 0 {
		return nil
	}

	req.SetUint16(req.Uint16() | unix.IFF_UP)
	if err := unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, req); err != nil {
		return fmt.Errorf("set %s flags: %w", loopbackInterfaceName, err)
	}

	return nil
}

func enterValidationChroot() error {
	// Validation reuses the normal nginx binary layout, so enter the prepared
	// chroot instead of rebuilding file paths for the child sandbox invocation.
	if err := unix.Chroot(validationChrootDir); err != nil {
		return fmt.Errorf("chroot %s: %w", validationChrootDir, err)
	}

	if err := unix.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root: %w", err)
	}

	return nil
}
