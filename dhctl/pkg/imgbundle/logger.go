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

package imgbundle

import (
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Logger struct{}

func (logger *Logger) DebugF(format string, a ...interface{}) {
	log.DebugLn(fmt.Sprintf(format, a...))
}

func (logger *Logger) DebugLn(a ...interface{}) {
	log.DebugLn(a)
}

func (logger *Logger) InfoF(format string, a ...interface{}) {
	log.InfoLn(fmt.Sprintf(format, a...))
}

func (logger *Logger) InfoLn(a ...interface{}) {
	log.InfoLn(a)
}

func (logger *Logger) WarnF(format string, a ...interface{}) {
	log.WarnLn(fmt.Sprintf(format, a...))
}

func (logger *Logger) WarnLn(a ...interface{}) {
	log.WarnLn(a)
}

func (logger *Logger) Process(topic string, run func() error) error {
	return log.Process("mirror", topic, run)
}
