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
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type Checker interface {
	IsReady() (bool, error)
	Name() string
}

func GetCheckers(kubeCl *client.KubernetesClient, resources template.Resources, metaConfig *config.MetaConfig) ([]Checker, error) {
	if metaConfig != nil {
		for _, terraNg := range metaConfig.GetTerraNodeGroups() {
			if terraNg.Replicas > 0 {
				checker := newClusterIsBootstrapCheck(&kubeNgGetter{kubeCl: kubeCl}, kubeCl)
				return []Checker{checker}, nil
			}
		}
	}
	errRes := &multierror.Error{}

	checkers := make([]Checker, 0)

	for _, r := range resources {
		check, err := tryToGetClusterIsBootstrappedChecker(kubeCl, r)
		if err != nil {
			errRes = multierror.Append(errRes, err)
			continue
		}

		if check != nil {
			checkers = append(checkers, check)
			// while we use one checker, we should break because
			// cluster is bootstrap checker should be in single instance
			// and should be single checker
			break
		}
	}

	if errRes.Len() > 0 {
		return nil, errRes
	}

	return checkers, nil
}

type Waiter struct {
	checkers []Checker
	attempts int
}

func NewWaiter(checkers []Checker) *Waiter {
	return &Waiter{
		attempts: 6,
		checkers: checkers,
	}
}

func (w *Waiter) WithAttempts(a int) *Waiter {
	w.attempts = a
	return w
}

func (w *Waiter) ReadyAll() (bool, error) {
	checkersToStay := make([]Checker, 0)

	for _, c := range w.checkers {
		var ready bool
		err := retry.NewSilentLoop(c.Name(), w.attempts, 5*time.Second).Run(func() error {
			var err error
			ready, err = c.IsReady()
			return err
		})

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
