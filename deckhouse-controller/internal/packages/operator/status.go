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

package operator

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
)

type Event struct {
	Namespace string
	Name      string
}

type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

func (o *Operator) setConditionTrue(app *Package, cond string) {
	for _, c := range app.status.Conditions {
		if c.Type == cond {
			c.Status = "True"
			o.notify(app.name)
			return
		}
	}

	app.status.Conditions = append(app.status.Conditions, Condition{
		Type:   cond,
		Status: "True",
	})

	o.notify(app.name)
}

func (o *Operator) notify(name string) {
	splits := strings.Split(name, ".")
	o.ch <- Event{
		Namespace: splits[0],
		Name:      splits[1],
	}
}

func (o *Operator) GetEventCh() <-chan Event {
	return o.ch
}

func (o *Operator) GetPackageCondition(namespace, name string) ([]Condition, error) {
	name = apps.BuildName(namespace, name)

	o.mu.Lock()
	defer o.mu.Unlock()

	app := o.packages[name]
	if app == nil {
		return nil, fmt.Errorf("application not found ")
	}

	return app.status.Conditions, nil
}
