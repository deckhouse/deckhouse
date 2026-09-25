/*
Copyright 2026 Flant JSC

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

// Command jq stands in for the real jq in the tests of this package, which execute the module's
// shell hooks the way shell-operator does. The unit test image ships no jq, while gojq is already
// a dependency, so TestMain builds this and puts it on PATH rather than adding a package to the
// image. It lives under testdata so that it is built only by those tests.
//
// It implements only what those hooks use — -r, -n, --arg, a program operand and input from a file
// or stdin — and refuses anything else instead of quietly behaving differently from jq.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "jqshim: %v\n", err)
		os.Exit(5)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	var (
		raw, nullInput bool
		names          []string
		values         []any
		program        string
		files          []string
	)

	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-r" || arg == "--raw-output":
			raw = true
		case arg == "-n" || arg == "--null-input":
			nullInput = true
		case arg == "--arg":
			if i+2 >= len(args) {
				return fmt.Errorf("--arg needs a name and a value")
			}
			names = append(names, "$"+args[i+1])
			values = append(values, args[i+2])
			i += 2
		case strings.HasPrefix(arg, "-") && arg != "-":
			return fmt.Errorf("unsupported option %q: this shim covers only what the hooks use", arg)
		case program == "":
			program = arg
		default:
			files = append(files, arg)
		}
	}

	if program == "" {
		return errors.New("no program given")
	}

	query, err := gojq.Parse(program)
	if err != nil {
		return fmt.Errorf("parsing the program: %w", err)
	}
	code, err := gojq.Compile(query, gojq.WithVariables(names))
	if err != nil {
		return fmt.Errorf("compiling the program: %w", err)
	}

	inputs := []any{nil}
	if !nullInput {
		if inputs, err = readInputs(files, stdin); err != nil {
			return err
		}
	}

	for _, input := range inputs {
		if err := emit(code.Run(input, values...), raw, stdout); err != nil {
			return err
		}
	}

	return nil
}

// readInputs decodes the stream of JSON values jq accepts, not a single document: the hooks pipe
// several objects into jq at once.
func readInputs(files []string, stdin io.Reader) ([]any, error) {
	var readers []io.Reader
	for _, name := range files {
		content, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		readers = append(readers, strings.NewReader(string(content)))
	}
	if len(readers) == 0 {
		readers = append(readers, stdin)
	}

	decoder := json.NewDecoder(io.MultiReader(readers...))
	var inputs []any
	for {
		var value any
		switch err := decoder.Decode(&value); {
		case errors.Is(err, io.EOF):
			return inputs, nil
		case err != nil:
			return nil, fmt.Errorf("decoding input: %w", err)
		}
		inputs = append(inputs, value)
	}
}

func emit(iter gojq.Iter, raw bool, stdout io.Writer) error {
	for {
		result, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, isErr := result.(error); isErr {
			return err
		}
		if text, isString := result.(string); isString && raw {
			if _, err := fmt.Fprintln(stdout, text); err != nil {
				return err
			}
			continue
		}
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
			return err
		}
	}
}
