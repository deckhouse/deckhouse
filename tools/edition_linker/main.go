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
	"fmt"
	"os"

	"edition_linker/conf"
	"edition_linker/linker"
)

func main() {
	if len(os.Args) != 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "merge":
		err := linker.MergeEditions(conf.MergeConf)
		if err != nil {
			fmt.Println(err)
			return
		}
	case "restore":
		err := linker.RestoreEditions(conf.MergeConf)
		if err != nil {
			fmt.Println(err)
			return
		}
	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println("This tool helps to create links between CE/EE/FE edition modules.\n" +
		"\tSyntax: go run tools/edition_linker (merge|restore)")
}
