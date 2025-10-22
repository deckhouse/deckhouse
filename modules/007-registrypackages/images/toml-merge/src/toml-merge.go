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
	"bufio"
	"fmt"
	"os"

	"toml-merge/internal/pkg/toml"
)

const version = "0.1"

func usage() {
	fmt.Printf(`Usage: %s SOURCE ... DEST
Merges several SOURCE toml files into DEST file.

Use '-' as DEST to write result to stdout.

Version %s.
Copyright 2023 Flant JSC.
Licensed under the Apache License, Version 2.0.
http://www.apache.org/licenses/LICENSE-2.0
`, os.Args[0])
}

func main() {
	if len(os.Args[1:]) < 2 {
		usage()
		os.Exit(0)
	}

	inFiles := os.Args[1 : len(os.Args)-1]
	outFile := os.Args[len(os.Args)-1]

	out, err := toml.Merge(inFiles)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var f *os.File
	if outFile == "-" {
		f = os.Stdout
	} else {
		f, err = os.OpenFile(outFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer f.Close()
	}

	writer := bufio.NewWriter(f)
	_, err = writer.Write(out)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = writer.Flush()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
