/*
Copyright 2025 Flant JSC

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

package edition

import (
	"fmt"
	"os"
	"strings"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition/verifier"
)

const (
	editionPath = "/deckhouse/edition"

	defaultBundle = "Default"

	bundleEnv = "DECKHOUSE_BUNDLE"
)

type Edition struct {
	verifier *verifier.Verifier
	Name     string
	Bundle   string
	Version  string
}

func Parse(version string) (*Edition, error) {
	content, err := os.ReadFile(editionPath)
	if err != nil {
		return nil, fmt.Errorf("read the '%s' edition file: %w", editionPath, err)
	}

	edition := new(Edition)
	edition.Version = version
	edition.Name = strings.ToLower(strings.TrimSpace(string(content)))
	edition.Bundle = strings.TrimSpace(os.Getenv(bundleEnv))
	if edition.Bundle == "" {
		edition.Bundle = defaultBundle
	}

	edition.verifier = verifier.New()

	return edition, nil
}

func (e *Edition) VerifyModule(module *moduletypes.Definition) error {
	return e.verifier.VerifyModule(module)
}

func (e *Edition) VerifySignature(module string, signature []byte) error {
	return e.verifier.VerifySignature(module, signature)
}
