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
	if err := enterValidationChroot(); err != nil {
		return err
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
