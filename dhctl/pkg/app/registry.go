// Copyright 2026 Flant JSC
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

package app

import (
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func DefineImgBundleFlags(cmd *kingpin.CmdClause, o *options.RegistryOptions) {
	cmd.Flag("img-bundle-path", `Path to the directory with tar or chunk.tar image bundles created by 'd8 mirror pull' command for Local registry mode.`).
		Envar(configEnvName("IMG_BUNDLE_PATH")).
		StringVar(&o.ImgBundlePath)
}
