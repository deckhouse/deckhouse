// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	ottrace "go.opentelemetry.io/otel/trace"
)

func StartApplication(ctx context.Context) (context.Context, ottrace.Span) {
	return StartSpan(
		ctx,
		"dhctl",
		ottrace.WithAttributes(
			attribute.String("service.instance.id", fmt.Sprintf("%s-%d", hostnameOrUnknown(), os.Getpid())),
			attribute.StringSlice("process.argv", append([]string(nil), os.Args...)),
			attribute.Int("process.pid", os.Getpid()),
			attribute.String("process.runtime.name", runtime.Version()),
			attribute.String("process.arch", runtime.GOARCH),
			attribute.String("process.os", runtime.GOOS),
			attribute.String("process.executable.path", executablePathOrUnknown()),
			attribute.String("process.working_directory", workingDirectoryOrUnknown()),
			attribute.String("process.owner", currentUserOrUnknown()),
		),
	)
}

func hostnameOrUnknown() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "unknown"
	}

	return hostname
}

func executablePathOrUnknown() string {
	path, err := os.Executable()
	if err != nil || path == "" {
		return "unknown"
	}

	return path
}

func workingDirectoryOrUnknown() string {
	dir, err := os.Getwd()
	if err != nil || dir == "" {
		return "unknown"
	}

	return dir
}

func currentUserOrUnknown() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}

	if u.Username != "" {
		return u.Username
	}

	if u.Uid != "" {
		return u.Uid
	}

	return "unknown"
}
