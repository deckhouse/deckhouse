// Copyright 2024 Flant JSC
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
	"flag"
	"log"
	"os"
	"syscall"

	"github.com/kolyshkin/capability"
)

var runDeckhouseController bool

func getCaps() (capability.Capabilities, error) {
	caps, err := capability.NewPid2(0)
	if err != nil {
		return nil, err
	}

	err = caps.Load()
	if err != nil {
		return nil, err
	}

	return caps, nil
}

func main() {
	flag.BoolVar(&runDeckhouseController, "runDeckhouseController", false, "Runs deckhouse-controller with the necessary capabilities.")
	flag.Parse()

	if runDeckhouseController {
		caps, err := getCaps()
		if err != nil {
			log.Fatal(err)
		}

		caps.Set(capability.INHERITABLE|capability.AMBIENT, capability.CAP_SYS_CHROOT, capability.CAP_SYS_ADMIN)

		err = caps.Apply(capability.CAPS | capability.AMBS)
		if err != nil {
			log.Fatal(err)
		}

		err = syscall.Exec("/usr/bin/tini", []string{"tini", "--", "/deckhouse/deckhouse-controller/entrypoint.sh"}, os.Environ())
		if err != nil {
			log.Fatal(err)
		}

		return
	}

	if len(os.Args) > 1 {
		caps, err := getCaps()
		if err != nil {
			log.Fatalf("getting capabilities: %s", err)
		}

		caps.Clear(capability.CAPS)

		err = caps.Apply(capability.CAPS)
		if err != nil {
			log.Fatalf("clearing capabilities: %s", err)
		}

		err = syscall.Exec(os.Args[1], os.Args[1:], os.Environ())
		if err != nil {
			log.Fatalf("executing the %q command: %s", os.Args[1:], err)
		}
	}
}
