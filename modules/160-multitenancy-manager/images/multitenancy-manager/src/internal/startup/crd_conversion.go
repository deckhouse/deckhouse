/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package startup

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CRDConversion waits until the CRDs the controller reads in more than one version convert through
// the conversion webhook.
//
// The module converts them with ConversionWebhook resources of its Helm release; the webhook-handler
// of the deckhouse module registers each of them on its CRD. Until then the CRD has strategy None,
// and the API server only rewrites apiVersion: a ClusterResourceGrantPolicy stored as v1alpha1 reads
// as v1alpha2 with its namespace selector in projectSelector. The CRD has None on an upgrade from a
// release where it had one version (the installer of the CRDs keeps the conversion it finds), and
// after the module is disabled and enabled again (the webhook-handler resets the conversion of a
// deleted ConversionWebhook). A controller that read the objects then would keep that reading in its
// caches and store it with its first write, so the controller process waits before it reads anything.
type CRDConversion struct {
	// Reader gets the CRDs. The uncached API reader: nothing else is read before the wait is over.
	Reader client.Reader
	Logger logr.Logger
	// Names are the names of the CRDs to wait for.
	Names []string
	// Interval is the pause between two checks.
	Interval time.Duration
}

// Wait blocks until every CRD of Names converts through the webhook and returns nil. A CRD that is
// not there is not waited for: nothing of it can be read. A failed check is retried at the next one;
// it is logged at error level once for each new error text, and the first check that succeeds after
// it leaves a line too, so an API server that stays unreachable does not fill the log. There is no
// timeout: the controller does not start on a reading it cannot trust. Wait returns the error of ctx
// when ctx is done first.
func (c *CRDConversion) Wait(ctx context.Context) error {
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	start := time.Now()
	var holding []string
	// lastErr is the text of the error the last check failed with, empty after a check that succeeded.
	var lastErr string
	for {
		pending, err := c.pending(ctx)
		switch {
		case err != nil && err.Error() != lastErr:
			c.Logger.Error(err, "cannot check the conversion of the CRDs, retrying")
			lastErr = err.Error()
		case err != nil:
			// The same error as at the last check, logged already.
		case lastErr != "":
			c.Logger.Info("the conversion of the CRDs can be checked again")
			lastErr = ""
		}

		switch {
		case err != nil:
			// Nothing is known about the CRDs; the next check tries again.
		case len(pending) == 0:
			// Every way out of the wait leaves a line, so the log tells whether the controller started.
			if holding != nil {
				c.Logger.Info("the CRDs convert through the webhook", "waited", time.Since(start).String())
			} else {
				c.Logger.Info("the CRDs already convert through the webhook")
			}
			return nil
		case !slices.Equal(pending, holding):
			c.Logger.Info("hold the controller until the webhook-handler of the deckhouse module registers "+
				"the conversion webhook of the CRDs", "crds", pending)
			holding = pending
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// pending returns the CRDs of Names that are there and do not convert through the webhook, each with
// its strategy, sorted.
func (c *CRDConversion) pending(ctx context.Context) ([]string, error) {
	var pending []string
	for _, name := range c.Names {
		crd := new(apiextensionsv1.CustomResourceDefinition)
		if err := c.Reader.Get(ctx, client.ObjectKey{Name: name}, crd); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get the CRD %s: %w", name, err)
		}

		strategy := apiextensionsv1.NoneConverter
		if crd.Spec.Conversion != nil && crd.Spec.Conversion.Strategy != "" {
			strategy = crd.Spec.Conversion.Strategy
		}
		if strategy != apiextensionsv1.WebhookConverter {
			pending = append(pending, fmt.Sprintf("%s (conversion strategy %s)", name, strategy))
		}
	}
	slices.Sort(pending)
	return pending, nil
}
