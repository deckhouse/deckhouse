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

// Package edition says which edition of Deckhouse Platform the controller is built for and what the
// edition limits.
//
// The edition is written into the binary when the image is built, from the edition werf builds
// (werf.inc.yaml of the image), the same value the edition file of the Deckhouse image comes from.
// Nothing in the cluster can change it: neither a value of a module nor an environment variable, so
// the only way to lift a limit is an image of another edition.
package edition

// Names of the editions that the controller tells apart.
const (
	// CE is Deckhouse Platform Open.
	CE = "CE"
	// Unknown is the edition of a binary built without one, by go build or go test.
	Unknown = "Unknown"
)

// built is the edition of the image, set by the build with
// -ldflags "-X controller/internal/edition.built=<EDITION>". It stays empty in any other build.
var built string

// Current returns the edition the binary is built for, Unknown when the build did not set one.
func Current() string {
	if built == "" {
		return Unknown
	}
	return built
}

// SetForTest makes Current return edition until the function it returns puts the built one back. It is
// for tests only, which must not run in parallel with other tests that read the edition.
func SetForTest(edition string) func() {
	saved := built
	built = edition
	return func() { built = saved }
}

// SingleNamespaceProjects reports whether a project of the edition has only its main namespace: no
// ProjectNamespace is created, and the controller makes and manages no additional namespace. Unknown
// is not limited: it is the edition of a developer build.
func SingleNamespaceProjects(edition string) bool {
	return edition == CE
}

// OnlyMainNamespace is what a message says in place of advice to create a ProjectNamespace, in an
// edition where a project has only its main namespace.
const OnlyMainNamespace = "In Deckhouse Platform Open (the CE edition), a project has only its main namespace. " +
	"Additional namespaces of a project are available in the commercial editions of Deckhouse Platform."

// SingleNamespaceMessage is what a refused or idle ProjectNamespace says.
const SingleNamespaceMessage = "In Deckhouse Platform Open (the CE edition), a project has only its main namespace: " +
	"a ProjectNamespace cannot be created, and the controller does not create or manage additional namespaces. " +
	"Additional namespaces of a project are available in the commercial editions of Deckhouse Platform."
