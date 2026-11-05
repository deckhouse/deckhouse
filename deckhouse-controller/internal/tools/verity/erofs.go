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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	tracerName = "verity"

	// fs type for mount
	erofsType = "erofs"

	// this is util to create erofs image
	mkfsCommand = "mkfs.erofs"

	// tarArg uses for tar input, it enables stream processing
	tarArg = "--tar=f"
	// aufsArg uses for AUFS-like layering for container images
	aufsArg = "--aufs"
	// quietArg disables logs
	quietArg = "--quiet"
	// noInlineArg disables data inlining for better compression/performance
	noInlineArg = "-Enoinline_data"

	// uClearArg uses for reusable builds
	uClearArg = "-Uclear"

	xArg = "-x-1"

	// staticTimestampArg uses for reusable builds
	staticTimestampArg = "-T 1750791050" // 2025-06-24T18:50:50Z
)

// ErrImageStream marks a failed read of the tar that CreateImageByTar builds the image from
var ErrImageStream = errors.New("read image stream")

// CreateImage uses mkfs.erofs to create image from module dir.
// Equivalent shell command:
// mkfs.erofs --aufs --quiet -Enoinline_data -T 1750791050 -Uclear -x-1 <imagePath> <modulePath>
func CreateImage(ctx context.Context, modulePath, imagePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateImage")
	defer span.End()

	span.SetAttributes(attribute.String("imagePath", imagePath))
	span.SetAttributes(attribute.String("modulePath", modulePath))

	args := []string{
		aufsArg,
		quietArg,
		noInlineArg,
		staticTimestampArg,
		uClearArg,
		xArg,

		imagePath,
		modulePath,
	}

	// mkfs.erofs --aufs --quiet -Enoinline_data -T 1750791050 -Uclear -x-1 <imagePath> <modulePath>
	cmd := exec.CommandContext(ctx, mkfsCommand, args...)
	output, err := combinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("create erofs image: %w (output: %s)", err, string(output))
	}

	return nil
}

// CreateImageByTar uses mkfs.erofs to create image from tar.
// A failed read of the tar kills mkfs.erofs, and the read error is returned.
// Equivalent shell command:
// mkfs.erofs --tar=f --aufs --quiet -Enoinline_data -T 1750791050 -Uclear -x-1 <imagePath> <modulePath>
func CreateImageByTar(ctx context.Context, rc io.ReadCloser, imagePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateImageByTar")
	defer span.End()

	span.SetAttributes(attribute.String("imagePath", imagePath))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream := &tarStream{reader: rc, cancel: cancel}

	args := []string{
		tarArg,
		aufsArg,
		quietArg,
		noInlineArg,
		staticTimestampArg,
		uClearArg,
		xArg,

		imagePath,
	}

	// mkfs.erofs --tar=f --aufs --quiet -Enoinline_data -T 1750791050 -Uclear -x-1 <imagePath> <modulePath>
	cmd := exec.CommandContext(ctx, mkfsCommand, args...)
	cmd.Stdin = stream

	output, err := combinedOutput(cmd)
	// Wait returns only after the stdin copier is done, so the stream error is safe to read.
	// It is the cause whether mkfs.erofs was killed or failed on the truncated tar
	if stream.err != nil {
		return fmt.Errorf("%w: %w", ErrImageStream, stream.err)
	}

	if err != nil {
		return fmt.Errorf("create erofs image: %w (output: %s)", err, string(output))
	}

	return nil
}

// tarStream feeds the tar to mkfs.erofs and kills it on the first failed read.
// exec reports a stdin copy error only after the process exits and drops it on
// a non-zero exit code, while mkfs.erofs can keep running on a truncated tar:
// erofs-utils before aa169fb ("fix CPU spin using --tar=f when stdin is closed")
// spins forever on a premature EOF.
type tarStream struct {
	reader io.Reader
	cancel context.CancelFunc

	// err is the failed read, set by the stdin copier
	err error
}

func (s *tarStream) Read(p []byte) (int, error) {
	n, err := s.reader.Read(p)
	// the copier ends the stream cleanly only on the io.EOF sentinel itself,
	// a wrapped io.EOF closes stdin as a failure too
	if err != nil && err != io.EOF {
		s.err = err
		s.cancel()
	}

	return n, err
}

// IsSupported reports whether the erofs+dm-verity module backend can be
// used. The result is computed once per process (memoized) via a real
// self-test rather than passive inspection.
func IsSupported() bool {
	return isSupported()
}

var isSupported = sync.OnceValue(func() bool {
	content, err := os.ReadFile("/proc/filesystems")
	if err != nil || !strings.Contains(string(content), erofsType) {
		return false
	}

	return selfTestDMVerity()
})

// selfTestDMVerity actually attempts to open a dm-verity mapping for a tiny
// scratch file, and reports whether that succeeded.
//
// This can't be answered by passively inspecting /proc or /sys: the kernel
// only autoloads the "verity" device-mapper target on a real table-load
// attempt (which is exactly what CreateMapper below does), not when merely
// listing already-loaded targets (e.g. via "dmsetup targets"). A kernel
// where dm-verity is a not-yet-loaded module would look unsupported to any
// such passive check, even though the real operation would succeed - the
// self-test avoids that false negative by doing the real thing.
func selfTestDMVerity() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dir, err := os.MkdirTemp("", "verity-selftest-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)

	imagePath := filepath.Join(dir, "test.img")
	if err := os.WriteFile(imagePath, make([]byte, 4096), 0o600); err != nil {
		return false
	}

	hash, err := CreateImageHash(ctx, imagePath)
	if err != nil {
		return false
	}

	name := fmt.Sprintf("verity-selftest-%d", os.Getpid())
	if err := CreateMapper(ctx, name, imagePath, hash); err != nil {
		return false
	}

	_ = CloseMapper(ctx, name)

	return true
}
