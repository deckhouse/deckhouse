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
	"fmt"
	"io"
	"os/exec"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	tracerName = "verity"

	// fs type for mount
	// nolint: unused
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create erofs image: %w (output: %s)", err, string(output))
	}

	return nil
}

// CreateImageByTar uses mkfs.erofs to create image from tar.
// Equivalent shell command:
// mkfs.erofs --tar=f --aufs --quiet -Enoinline_data -T 1750791050 -Uclear -x-1 <imagePath> <modulePath>
func CreateImageByTar(ctx context.Context, rc io.ReadCloser, imagePath string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateImageByTar")
	defer span.End()

	span.SetAttributes(attribute.String("imagePath", imagePath))

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
	cmd.Stdin = rc

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create erofs image: %w (output: %s)", err, string(output))
	}

	return nil
}
