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

package main

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

var allowedCommands []string

type Command struct {
	Name       string
	Help       string
	DefineFunc func(cmd *kingpin.CmdClause) *kingpin.CmdClause
	Parrent    string
	cmd        *kingpin.CmdClause
}

func checkCommand(name string, allowedCommands []string) (bool, []string) {
	if len(allowedCommands) == 0 || slices.Index(allowedCommands, name) != -1 {
		return true, []string{}
	}

	for _, cm := range allowedCommands {
		c := strings.Split(cm, " ")
		if c[0] == name {
			return true, c
		}
	}

	return false, []string{}
}

func checkSubcommand(name string, subcommands []string) bool {
	ex, _ := checkCommand(name, subcommands)
	if len(subcommands) == 2 && subcommands[1] == "*" || ex {
		return true
	}

	return false
}

func getParentIndex(commandList []Command, name string) (int, error) {
	for i, cmd := range commandList {
		if name == cmd.Name {
			return i, nil
		}
	}

	return -1, fmt.Errorf("parrent command %s not found in command list", name)
}

func getNestingDepth(cmd Command, commands []Command) (Command, int) {
	depth := 0
	visited := make(map[string]bool)
	topLevel := cmd

	for {
		found := false
		for _, c := range commands {
			if c.Name == cmd.Parrent && !visited[c.Name] {
				visited[c.Name] = true
				cmd = c
				depth++
				topLevel = cmd
				found = true
				break
			}
		}

		if !found || cmd.Parrent == "" {
			break
		}
	}

	return topLevel, depth
}

func initParent(parrentCmdIndex int, kpApp *kingpin.Application) *kingpin.CmdClause {
	var pcmd *kingpin.CmdClause

	if commandList[parrentCmdIndex].cmd == nil {
		pcmd = kpApp.Command(commandList[parrentCmdIndex].Name, commandList[parrentCmdIndex].Help)
		commandList[parrentCmdIndex].cmd = pcmd
	} else {
		pcmd = commandList[parrentCmdIndex].cmd
	}
	return pcmd
}

func registerCommands(kpApp *kingpin.Application) error {
	for i, command := range commandList {
		firstNode, depth := getNestingDepth(command, commandList)
		if depth == 0 {
			allowed, _ := checkCommand(command.Name, allowedCommands)
			if allowed {
				cmd := kpApp.Command(command.Name, command.Help)
				commandList[i].cmd = cmd

				if command.DefineFunc != nil {
					command.DefineFunc(cmd)
				}
			}
		} else {
			parrentCmdIndex, err := getParentIndex(commandList, command.Parrent)
			if err != nil {
				return err
			}

			allowed, subcommands := checkCommand(firstNode.Name, allowedCommands)

			if allowed && checkSubcommand(command.Name, subcommands) {
				pcmd := initParent(parrentCmdIndex, kpApp)

				cmd := pcmd.Command(command.Name, command.Help)
				commandList[i].cmd = cmd

				if command.DefineFunc != nil {
					command.DefineFunc(cmd)
				}
			}
		}
	}

	return nil
}
