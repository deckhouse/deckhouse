//go:build linux

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

package verity

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/unix"
)

const (
	dmTemplate = "/dev/mapper/%s"
)

// Mount ensures the mount path and mounts the device mapper to it
func Mount(ctx context.Context, name, mountPath string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Mount")
	defer span.End()

	// /dev/mapper/<name>
	dmPath := fmt.Sprintf(dmTemplate, name)

	span.SetAttributes(attribute.String("mapper", dmPath))
	span.SetAttributes(attribute.String("path", mountPath))

	if err := ensureMountDir(mountPath); err != nil {
		return fmt.Errorf("create the path '%s': %w", mountPath, err)
	}

	return unix.Mount(dmPath, mountPath, erofsType, unix.MS_RDONLY, "")
}

// ensureMountDir makes mountPath a directory, replacing a leftover non-directory entry such as a symlink.
func ensureMountDir(mountPath string) error {
	info, err := os.Lstat(mountPath)
	if err == nil && info.IsDir() {
		return nil
	}

	if err == nil {
		if err = os.Remove(mountPath); err != nil {
			return fmt.Errorf("remove non-directory: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.MkdirAll(mountPath, 0755)
}

// Unmount unmounts image and remove mount path
func Unmount(ctx context.Context, mountPath string) error {
	_, span := otel.Tracer(tracerName).Start(ctx, "Unmount")
	defer span.End()

	span.SetAttributes(attribute.String("path", mountPath))

	// Lstat, so a dangling symlink is seen and removed instead of treated as absent
	info, err := os.Lstat(mountPath)
	if os.IsNotExist(err) {
		return nil
	}

	// a symlink is never a mount point, it is a leftover of the symlink backend
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(mountPath)
	}

	// unmount to /deckhouse/downloaded/modules/<module>
	if err = unix.Unmount(mountPath, unix.UMOUNT_NOFOLLOW); err != nil {
		// if we get this error, it means the target is not mount so just delete it
		if !errors.Is(err, unix.EINVAL) {
			return fmt.Errorf("unmount the path '%s' : %w", mountPath, err)
		}
	}

	return os.RemoveAll(mountPath)
}
