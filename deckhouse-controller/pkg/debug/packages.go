// Copyright 2025 Flant JSC
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

package debug

import (
	"context"
	"fmt"
	"net/url"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug"
)

var packagesDebugSocket = "/tmp/deckhouse-debug.socket"

func DefinePackagesCommands(kpApp *kingpin.Application) {
	packagesCmd := kpApp.Command("packages", "Package debug commands.")

	var packageName string

	packagesDumpCmd := packagesCmd.Command("dump", "Dump all/specific packages state from memory.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			ctx := context.Background()
			out, err := client.Get(ctx, withQuery("packages/dump", "name", packageName))
			if err != nil {
				return err
			}
			fmt.Println(string(out))

			return nil
		})
	packagesDumpCmd.Flag("name", "Filter by package name.").StringVar(&packageName)
	definePackagesDebugSocketFlag(packagesDumpCmd)

	schedulerCmd := packagesCmd.Command("scheduler", "Scheduler operations.")
	schedulerDumpCmd := schedulerCmd.Command("dump", "Dump all scheduler node state from memory.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			ctx := context.Background()
			out, err := client.Get(ctx, withQuery("packages/scheduler/dump", "name", packageName))
			if err != nil {
				return err
			}
			fmt.Println(string(out))

			return nil
		})
	schedulerDumpCmd.Flag("name", "Filter by package name.").StringVar(&packageName)
	definePackagesDebugSocketFlag(schedulerDumpCmd)

	packagesQueueCmd := packagesCmd.Command("queue", "Queue operations.")
	packagesQueueListCmd := packagesQueueCmd.Command("dump", "Dump all package queues with tasks.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			ctx := context.Background()
			out, err := client.Get(ctx, withQuery("packages/queues/dump", "name", packageName))
			if err != nil {
				return err
			}
			fmt.Println(string(out))

			return nil
		})
	packagesQueueListCmd.Flag("name", "Filter by package name.").StringVar(&packageName)
	definePackagesDebugSocketFlag(packagesQueueListCmd)

	packagesRenderCmd := packagesCmd.Command("render", "Render package Helm templates.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			ctx := context.Background()
			out, err := client.Get(ctx, "packages/render", packageName)
			if err != nil {
				return err
			}
			fmt.Println(string(out))

			return nil
		})
	packagesRenderCmd.Arg("package_name", "Name of the package to render.").Required().StringVar(&packageName)
	definePackagesDebugSocketFlag(packagesRenderCmd)

	packagesSnapshotsCmd := packagesCmd.Command("snapshots", "Dump hook snapshots for a package.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			ctx := context.Background()
			out, err := client.Get(ctx, "packages/snapshots", packageName)
			if err != nil {
				return err
			}
			fmt.Println(string(out))

			return nil
		})
	packagesSnapshotsCmd.Arg("package_name", "Name of the package.").Required().StringVar(&packageName)
	definePackagesDebugSocketFlag(packagesSnapshotsCmd)
}

func definePackagesDebugSocketFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("debug-unix-socket", "Path to Unix socket for packages debug endpoint.").
		Envar("PACKAGES_DEBUG_UNIX_SOCKET").
		Default(packagesDebugSocket).
		StringVar(&packagesDebugSocket)
}

// withQuery appends a query parameter to a path if value is non-empty.
func withQuery(path, key, value string) string {
	if value == "" {
		return path
	}
	return path + "?" + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}
