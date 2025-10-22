/*
Copyright 2023 Flant JSC

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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	alerttemplates "tools/helm_generate/runners/alert_templates"
	authzgeneraterulesforroles "tools/helm_generate/runners/authz_generate_rules_for_roles"
	"tools/helm_generate/runners/conversion"
)

type Runner interface {
	Init([]string) error
	Run() error
	Name() string
}

func run(args []string) error {
	if len(args) < 1 {
		return errors.New("you must pass a runner name")
	}

	cmds := []Runner{
		alerttemplates.NewImageChecks(),
		authzgeneraterulesforroles.NewAuthzGenerate(),
		conversion.NewConversion(),
	}

	subcommand := os.Args[1]

	for _, cmd := range cmds {
		if cmd.Name() == subcommand {
			if err := cmd.Init(os.Args[2:]); errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return cmd.Run()
		}
	}

	return fmt.Errorf("unknown runner name: %s", subcommand)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
