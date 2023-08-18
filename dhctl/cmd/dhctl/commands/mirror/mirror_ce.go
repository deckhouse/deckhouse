//go:build ce

// Copyright 2023 Flant JSC
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

package mirror

import (
	"errors"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

func DefineMirrorCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("mirror", "Copy images from deckhouse registry or tar.gz file to specified registry or tar.gz file.")
	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("mirror", "Copy images", func() error {
			return errors.New("dhctl mirror can't be used in CE edition")
		})
	})
	return cmd
}
