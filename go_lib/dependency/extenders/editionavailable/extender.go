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

package editionavailable

import (
	"fmt"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/go_lib/d8edition"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name extenders.ExtenderName = "EditionAvailable"
)

var (
	instance *Extender
	once     sync.Once
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	edition *d8edition.Edition
	logger  *log.Logger
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{}
	})
	return instance
}

func (e *Extender) SetLogger(logger *log.Logger) *Extender {
	e.logger = logger.Named("extender-edition-available")
	return e
}

func (e *Extender) SetEdition(edition *d8edition.Edition) {
	e.edition = edition
}

// Name implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool {
	return true
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	available := e.edition.IsAvailable(d8edition.Embedded, name)
	if available == nil {
		e.logger.Debugf("the '%s' module skipped", name)
		return nil, nil
	}
	if *available {
		e.logger.Debugf("the '%s' module available in the '%s' edition", name, e.edition.String())
		return available, nil
	}
	e.logger.Warnf("the '%s' module is unavailable in the '%s' edition", name, e.edition.String())
	return available, fmt.Errorf("unavailable in the '%s' edition", e.edition.String())
}
