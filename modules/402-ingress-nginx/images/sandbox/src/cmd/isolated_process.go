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

const validationChrootDir = "/validation-chroot"

func setupIsolatedProcessChild() error {
	// Validation reuses the normal nginx binary layout, so enter the prepared
	// chroot instead of rebuilding file paths for the child sandbox invocation.
	if err := unix.Chroot(validationChrootDir); err != nil {
		return fmt.Errorf("chroot %s: %w", validationChrootDir, err)
	}

	if err := unix.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root: %w", err)
	}

	// The helper only needs CAP_SYS_CHROOT long enough to enter the validation
	// root. Drop it before starting the ptraced target so nginx cannot inherit it.
	if err := dropCapabilityFromCurrentProcess(unix.CAP_SYS_CHROOT); err != nil {
		return fmt.Errorf("drop CAP_SYS_CHROOT after entering validation root: %w", err)
	}

	return nil
}

func dropCapabilityFromCurrentProcess(cap uint) error {
	word, mask, err := capabilityWordMask(cap)
	if err != nil {
		return err
	}

	if err := lowerAmbientCapability(cap); err != nil {
		return err
	}

	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{}
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return err
	}

	data[word].Effective &^= mask
	data[word].Permitted &^= mask
	data[word].Inheritable &^= mask

	return unix.Capset(&hdr, &data[0])
}

func lowerAmbientCapability(cap uint) error {
	isSet, err := unix.PrctlRetInt(
		unix.PR_CAP_AMBIENT,
		uintptr(unix.PR_CAP_AMBIENT_IS_SET),
		uintptr(cap),
		0,
		0,
	)
	if err != nil {
		return err
	}
	if isSet == 0 {
		return nil
	}

	return unix.Prctl(
		unix.PR_CAP_AMBIENT,
		uintptr(unix.PR_CAP_AMBIENT_LOWER),
		uintptr(cap),
		0,
		0,
	)
}

func capabilityWordMask(cap uint) (int, uint32, error) {
	word := int(cap / 32)
	if word >= 2 {
		return 0, 0, fmt.Errorf("capability %d out of range", cap)
	}

	return word, uint32(1) << (cap % 32), nil
}
