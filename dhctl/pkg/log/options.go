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
	"github.com/fatih/color"
	"github.com/flant/logboek"
)

func BootstrapOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgYellow, color.Bold},
			},
		},
	}
}

func CommonOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgBlue, color.Bold},
			},
		},
	}
}

func boldStyle() *logboek.Style {
	return &logboek.Style{
		Attributes: []color.Attribute{color.Bold},
	}
}

func BoldOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: boldStyle(),
		},
	}
}

func BoldStartOptions() logboek.LogProcessStartOptions {
	return logboek.LogProcessStartOptions{
		LevelLogProcessStartOptions: logboek.LevelLogProcessStartOptions{
			Style: boldStyle(),
		},
	}
}

func BoldEndOptions() logboek.LogProcessEndOptions {
	return logboek.LogProcessEndOptions{
		LevelLogProcessEndOptions: logboek.LevelLogProcessEndOptions{
			Style: boldStyle(),
		},
	}
}

func BoldFailOptions() logboek.LogProcessFailOptions {
	return logboek.LogProcessFailOptions{
		LevelLogProcessFailOptions: logboek.LevelLogProcessFailOptions{
			LevelLogProcessEndOptions: logboek.LevelLogProcessEndOptions{
				Style: boldStyle(),
			},
		},
	}
}

func TerraformOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{LevelLogProcessOptions: logboek.LevelLogProcessOptions{
		Style: &logboek.Style{
			Attributes: []color.Attribute{color.FgGreen, color.Bold},
		},
	}}
}

func ConvergeOptions() logboek.LogProcessOptions {
	return logboek.LogProcessOptions{
		LevelLogProcessOptions: logboek.LevelLogProcessOptions{
			Style: &logboek.Style{
				Attributes: []color.Attribute{color.FgHiCyan, color.Bold},
			},
		},
	}
}
