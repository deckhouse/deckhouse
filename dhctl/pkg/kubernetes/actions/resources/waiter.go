// Copyright 2022 Flant JSC
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

package resources

import (
	"context"
	"reflect"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type Checker interface {
	IsReady(ctx context.Context) (bool, error)
	Name() string
	Single() bool
}

func GetCheckers(kubeCl *client.KubernetesClient, resources template.Resources, metaConfig *config.MetaConfig) ([]Checker, error) {
	errRes := &multierror.Error{}

	checkers := make([]Checker, 0)
	singleConstructors := make(map[string]interface{})

	tryToAppendCheck := func(check Checker, err error) {
		if err != nil {
			errRes = multierror.Append(errRes, err)
			return
		}

		if check == nil || reflect.ValueOf(check).IsNil() {
			return
		}

		_, hasSingleCheck := singleConstructors[check.Name()]
		if !check.Single() || !hasSingleCheck {
			checkers = append(checkers, check)
			singleConstructors[check.Name()] = struct{}{}
		}
	}

	staticNGSChecker, err := tryToGetClusterIsBootstrappedCheckerFromStaticNGS(kubeCl, metaConfig)
	tryToAppendCheck(staticNGSChecker, err)

	type constructor func(*client.KubernetesClient, *config.MetaConfig, *template.Resource) (Checker, error)

	constructors := []constructor{
		tryToGetClusterIsBootstrappedChecker,
		tryToGetResourceIsReadyChecker,
	}

	for _, r := range resources {
		for _, crtor := range constructors {
			check, err := crtor(kubeCl, metaConfig, r)
			tryToAppendCheck(check, err)
		}
	}

	if errRes.Len() > 0 {
		return nil, errRes
	}

	return checkers, nil
}

type Waiter struct {
	checkers []Checker
}

func NewWaiter(checkers []Checker) *Waiter {
	return &Waiter{
		checkers: checkers,
	}
}

func (w *Waiter) ReadyAll(ctx context.Context) (bool, error) {
	checkersToStay := make([]Checker, 0)

	for _, c := range w.checkers {
		ready, err := c.IsReady(ctx)
		if err != nil {
			return false, err
		}

		if !ready {
			checkersToStay = append(checkersToStay, c)
		}
	}

	w.checkers = checkersToStay

	return len(w.checkers) == 0, nil
}
