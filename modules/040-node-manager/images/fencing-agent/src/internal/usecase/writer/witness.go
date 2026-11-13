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

package writer

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// Attest takes an accepted DeclareSuicide of node. The witness writes its
// attestation in the FFNS of node. When the FFNS does not exist, the witness
// first creates it with the source DeclareSuicide from the last NodeMeta of
// node. A witness in maintenance does not create and does not change the FFNS.
func (w *Writer) Attest(node string) {
	at := time.Now()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return
	}

	w.spawn(func(ctx context.Context) { w.attest(ctx, node, at) })
}

func (w *Writer) attest(ctx context.Context, node string, at time.Time) {
	// witness is the witness of an attestation that exists.
	var witness string

	err := attempt(ctx, func(ctx context.Context) error {
		var err error
		witness, err = w.attestOnce(ctx, node, at)

		return err
	}, func(n int, err error) {
		w.logger.Warn("write the attestation", "peer", node, "attempt", n, "error", err)
	})

	switch {
	case errors.Is(err, errNotAllowed):
		w.logger.Info("no attestation from this witness", "peer", node, "reason", err.Error())
	case err == nil && witness != "":
		w.logger.Info("no attestation from this witness: the FFNS is already attested", "peer", node, "witness", witness)
	case err == nil:
		w.logger.Info("attestation written", "peer", node)
	}
}

// attestOnce is one attempt of attest. When the FFNS already has an
// attestation, it writes nothing and returns the witness of that attestation.
func (w *Writer) attestOnce(ctx context.Context, node string, at time.Time) (string, error) {
	if w.deps.Maintenance() {
		return "", fmt.Errorf("%w: the own node is in maintenance", errNotAllowed)
	}

	ffns, err := w.deps.API.GetFFNS(ctx, node)
	if apierrors.IsNotFound(err) {
		ffns, err = w.createAttested(ctx, node, at)
	}

	if err != nil {
		return "", err
	}

	if a := ffns.Status.Attestation; a != nil {
		return a.Witness, nil
	}

	attested := ffns.DeepCopy()
	attested.Status.Attestation = &v1alpha1.FencingFailedNodeStateAttestation{Witness: w.deps.Self, At: metav1.NewMicroTime(at)}

	return "", w.deps.API.PatchStatus(ctx, ffns, attested)
}

// createAttested creates the FFNS of node with the source DeclareSuicide. The
// API server drops the status of a new object, so the attestation is a
// separate write. On AlreadyExists it returns the FFNS that exists.
func (w *Writer) createAttested(ctx context.Context, node string, at time.Time) (*v1alpha1.FencingFailedNodeState, error) {
	var meta domain.NodeMeta

	w.mu.Lock()
	if p := w.peers[node]; p != nil {
		meta = p.meta
	}
	w.mu.Unlock()

	if meta.BootID == "" {
		return nil, fmt.Errorf("%w: no NodeMeta of the peer", errNotAllowed)
	}

	if meta.Maintenance {
		return nil, fmt.Errorf("%w: the peer is in maintenance", errNotAllowed)
	}

	ffns, err := w.newFFNS(node, meta, v1alpha1.SourceDeclareSuicide, at)
	if err != nil {
		return nil, err
	}

	err = w.deps.API.CreateFFNS(ctx, ffns)
	if apierrors.IsAlreadyExists(err) {
		return w.deps.API.GetFFNS(ctx, node)
	}

	return ffns, err
}
