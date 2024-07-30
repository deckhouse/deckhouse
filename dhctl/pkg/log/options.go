// Copyright 2021 Flant JSC
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

package log

import (
	"github.com/gookit/color"
	"github.com/werf/logboek/pkg/types"
)

func BootstrapOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgYellow, color.Bold))
}

func MirrorOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgGreen, color.Bold))
}

func CommanderAttachOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgLightCyan, color.Bold))
}

func CommanderDetachOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgLightCyan, color.Bold))
}

func CommonOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgBlue, color.Bold))
}

func BoldOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func BoldStartOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func BoldEndOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func BoldFailOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(boldStyle())
}

func TerraformOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgGreen, color.Bold))
}

func ConvergeOptions(opts types.LogProcessOptionsInterface) {
	opts.Style(color.New(color.FgLightCyan, color.Bold))
}

func boldStyle() color.Style {
	return color.New(color.Bold)
}
