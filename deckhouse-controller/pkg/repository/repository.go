// Copyright 2022 Flant JSC
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

package repository

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	listReleases = false
)

func DefineRepositoryCommand(kpApp *kingpin.Application) {
	repositoryCmd := kpApp.Command("repository", "Deckhouse repository work.").PreAction(func(context *kingpin.ParseContext) error {
		kpApp.UsageTemplate(kingpin.DefaultUsageTemplate)
		return nil
	})

	repositoryListCmd := repositoryCmd.Command("list", "List in registry").
		Action(func(c *kingpin.ParseContext) error {
			fmt.Println("listing releases", listReleases)
			return nil
		})

	repositoryListCmd.Flag("releases", "Show releases list.").Short('e').
		Default("false").
		BoolVar(&listReleases)
}
