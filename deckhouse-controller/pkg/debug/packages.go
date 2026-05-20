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
	"os"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug"
)

var packagesDebugSocket = "/tmp/deckhouse-debug.socket"

func DefinePackagesCommands(rootCmd *cobra.Command) {
	packagesCmd := &cobra.Command{
		Use:   "packages",
		Short: "Package debug commands.",
	}
	rootCmd.AddCommand(packagesCmd)

	{
		var packageName string
		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump all/specific packages state from memory.",
			RunE: func(_ *cobra.Command, _ []string) error {
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
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		definePackagesDebugSocketFlag(dumpCmd)
		packagesCmd.AddCommand(dumpCmd)
	}

	{
		schedulerCmd := &cobra.Command{Use: "scheduler", Short: "Scheduler operations."}
		packagesCmd.AddCommand(schedulerCmd)

		var packageName string
		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump all scheduler node state from memory.",
			RunE: func(_ *cobra.Command, _ []string) error {
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
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		definePackagesDebugSocketFlag(dumpCmd)
		schedulerCmd.AddCommand(dumpCmd)
	}

	{
		queueCmd := &cobra.Command{Use: "queue", Short: "Queue operations."}
		packagesCmd.AddCommand(queueCmd)

		var packageName string
		dumpCmd := &cobra.Command{
			Use:   "dump",
			Short: "Dump all package queues with tasks.",
			RunE: func(_ *cobra.Command, _ []string) error {
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
			},
		}
		dumpCmd.Flags().StringVar(&packageName, "name", "", "Filter by package name.")
		definePackagesDebugSocketFlag(dumpCmd)
		queueCmd.AddCommand(dumpCmd)
	}

	{
		renderCmd := &cobra.Command{
			Use:   "render PACKAGE_NAME",
			Short: "Render package Helm templates.",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				client, err := debug.NewClient(packagesDebugSocket)
				if err != nil {
					return err
				}
				defer client.Close()

				ctx := context.Background()
				out, err := client.Get(ctx, "packages/render", args[0])
				if err != nil {
					return err
				}
				fmt.Println(string(out))

				return nil
			},
		}
		definePackagesDebugSocketFlag(renderCmd)
		packagesCmd.AddCommand(renderCmd)
	}

	{
		snapshotsCmd := &cobra.Command{
			Use:   "snapshots PACKAGE_NAME",
			Short: "Dump hook snapshots for a package.",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				client, err := debug.NewClient(packagesDebugSocket)
				if err != nil {
					return err
				}
				defer client.Close()

				ctx := context.Background()
				out, err := client.Get(ctx, "packages/snapshots", args[0])
				if err != nil {
					return err
				}
				fmt.Println(string(out))

				return nil
			},
		}
		definePackagesDebugSocketFlag(snapshotsCmd)
		packagesCmd.AddCommand(snapshotsCmd)
	}
}

func definePackagesDebugSocketFlag(cmd *cobra.Command) {
	defaultSocket := packagesDebugSocket
	if v, ok := os.LookupEnv("PACKAGES_DEBUG_UNIX_SOCKET"); ok && v != "" {
		defaultSocket = v
		packagesDebugSocket = v
	}
	cmd.Flags().StringVar(&packagesDebugSocket, "debug-unix-socket", defaultSocket, "Path to Unix socket for packages debug endpoint.")
}

// withQuery appends a query parameter to a path if value is non-empty.
func withQuery(path, key, value string) string {
	if value == "" {
		return path
	}
	return path + "?" + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}
