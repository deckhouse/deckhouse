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
	"fmt"

	sh_debug "github.com/flant/shell-operator/pkg/debug"
	"gopkg.in/alecthomas/kingpin.v2"
)

var packagesDebugSocket = "/tmp/deckhouse-debug.socket"

func DefinePackagesCommands(kpApp *kingpin.Application) {
	packagesCmd := kpApp.Command("packages", "Package debug commands.")

	packagesDumpCmd := packagesCmd.Command("dump", "Dump all packages state from memory.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := sh_debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			out, err := client.Get("http://unix/packages/dump")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		})
	definePackagesDebugSocketFlag(packagesDumpCmd)

	packagesQueueCmd := packagesCmd.Command("queue", "Queue operations.")
	packagesQueueListCmd := packagesQueueCmd.Command("list", "List all package queues with tasks.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := sh_debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			out, err := client.Get("http://unix/packages/queues/dump")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		})
	definePackagesDebugSocketFlag(packagesQueueListCmd)

	var packageName string
	packagesRenderCmd := packagesCmd.Command("render", "Render package Helm templates.").
		Action(func(_ *kingpin.ParseContext) error {
			client, err := sh_debug.NewClient(packagesDebugSocket)
			if err != nil {
				return err
			}
			defer client.Close()

			out, err := client.Get(fmt.Sprintf("http://unix/packages/render/%s", packageName))
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		})
	packagesRenderCmd.Arg("package_name", "Name of the package to render.").Required().StringVar(&packageName)
	definePackagesDebugSocketFlag(packagesRenderCmd)
}

func definePackagesDebugSocketFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("debug-unix-socket", "Path to Unix socket for packages debug endpoint.").
		Envar("PACKAGES_DEBUG_UNIX_SOCKET").
		Default(packagesDebugSocket).
		StringVar(&packagesDebugSocket)
}
