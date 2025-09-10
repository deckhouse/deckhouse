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

package fsstatic

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type CheckLinkSource func(string) error

func CreateLinkIfNotExists(source string, check CheckLinkSource, destination string, logger log.Logger) error {
	logger.LogDebugF("Create link from %s to %s\n", source, destination)

	link, err := os.Readlink(destination)
	if err == nil {
		if link == source {
			logger.LogDebugF("Link %s exists and have valid source %s\n", destination, source)
			return nil
		}

		logger.LogDebugF("Link %s exists, but do not have source %s, source is %s Remove and recreate\n",
			destination,
			source,
			link,
		)

		err = os.Remove(destination)
		if err != nil {
			return fmt.Errorf("Cannot remove link %s: %w", destination, err)
		}
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("Cannot read link %s: %w", destination, err)
	}

	if err := check(source); err != nil {
		return fmt.Errorf("Cannot create link %s to %s: %w", source, destination, err)
	}

	if err := os.Symlink(source, destination); err != nil {
		return fmt.Errorf("Cannot create link %s to %s: %w", source, destination, err)
	}

	return nil
}
