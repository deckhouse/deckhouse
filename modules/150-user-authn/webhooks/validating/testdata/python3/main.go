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

// Command python3 stands in for the real python3 in the tests of this package. It is not an
// interpreter: it emulates the single program the module's hooks run under python3, the ipaddress
// check of the validating webhook, and answers it with net.ParseIP. It lives under testdata so
// that it is built only by those tests.
//
// The hook discards this program's output and reads only its exit status, so a refusal here would
// be invisible. The caller therefore checks that the hook's Python still matches what this
// emulates before putting the shim on PATH.
package main

import (
	"io"
	"net"
	"os"
)

func main() {
	// The hook invokes "python3 - <value>", passing the program on stdin. Drain it so the writing
	// side never blocks on a full pipe.
	_, _ = io.Copy(io.Discard, os.Stdin)

	args := os.Args[1:]
	if len(args) != 2 || args[0] != "-" {
		os.Exit(2)
	}

	if net.ParseIP(args[1]) == nil {
		os.Exit(1)
	}
}
