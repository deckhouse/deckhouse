package resources

import (
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type Checker interface {
	IsReady() (bool, error)
	Name() string
}

func GetCheckers(kubeCl *client.KubernetesClient, resources template.Resources) ([]Checker, error) {
	errRes := &multierror.Error{}

	checkers := make([]Checker, 0)

	for _, r := range resources {
		check, err := TryToGetEphemeralNodeGroupChecker(kubeCl, r)
		if err != nil {
			errRes = multierror.Append(errRes, err)
			continue
		}

		if check != nil {
			checkers = append(checkers, check)
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

func (w *Waiter) Step() (bool, error) {
	checkersToStay := make([]Checker, 0)

	for _, c := range w.checkers {
		var ready bool
		err := retry.NewLoop(c.Name(), 5, 3*time.Second).Run(func() error {
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
