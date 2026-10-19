// Copyright 2026 Flant JSC
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

// The preparation of a candidate revision (attach, load): an operation the package Reconciler
// starts, cancels and retries; Restore runs the same steps once, in place.

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/gate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// stepAttach downloads, verifies and mounts the image under its instance, in one backend call:
	// verification needs the registry anyway (the root hash).
	stepAttach = "attach"
	stepLoad   = "load" // stepLoad loads the package object from the attached image.
)

// backend is a disk backend (erofs, symlink); every call blocks. It keys mounts by instance
// and stores an image per version, so Current and a Candidate coexist. Detach never deletes a
// stored image: the package's versions, the Candidate's included, share one directory; only the
// cleanup before boot purges files.
type backend interface {
	// Attach downloads the image unless the stored one verifies, and mounts it under instance.
	Attach(ctx context.Context, repo registry.Remote, instance, packageName, version string) error
	// Detach unmounts instance and keeps the stored image. Nil when not attached.
	Detach(ctx context.Context, instance string) error
}

// Disk is a backend with its working directory; an instance is mounted at
// disk.AttachPath(Dir, instance).
type Disk struct {
	Backend backend
	Dir     string
}

// Disks are the disks of every kind, shared by every package Reconciler. Known limit, accepted: a
// backend holds its mutex during a download, so a detach can wait behind another package's
// download.
type Disks struct {
	Apps    Disk
	Modules Disk
	// EmbeddedDir is the deckhouse image directory: an embedded module's image is
	// EmbeddedDir/<name> (the loader resolves the weight prefix). Nothing is attached there.
	EmbeddedDir string
}

// LoadFunc loads a new package object of kind from an attached image; it reads only the image,
// never the cluster. The name is image.Name, never read from the path, which carries the version
// (name@version).
type LoadFunc func(ctx context.Context, kind model.Kind, image model.Image) (model.Package, error)

// target is a preparation's identity, whole: a corrected source must restart a preparation that
// keeps failing with the old one.
type target struct {
	Kind     model.Kind
	Name     string
	Version  string
	Source   model.Source
	Embedded bool
}

// instance is the attach instance, name@version. The version is part of it, so a Candidate's image
// never replaces Current's. A version never contains "@": the loader takes it from the mount path
// after the first "@".
func instance(name, version string) string {
	return name + "@" + version
}

// prepared is what a preparation produced so far. Its steps write it; the package Reconciler reads
// it only after the operation's Done delivered, when the operation goroutine has exited.
type prepared struct {
	image  *model.Image  // image is nil until attached.
	object model.Package // object is nil until loaded.
}

// revision is valid only once object is set.
func (p *prepared) revision() model.Revision {
	return model.Revision{Image: *p.image, Package: p.object}
}

// prepareSteps attaches and loads t into st, skipping what st already holds: a retry continues
// where the last attempt failed, so an attached image is not attached and verified again.
func prepareSteps(deps Deps, t target, st *prepared) []operation.Step {
	return []operation.Step{
		{Name: stepAttach, Reason: model.ReasonAttachFailed, Run: func(ctx context.Context) error {
			if st.image != nil {
				return nil
			}

			img, err := attach(ctx, deps.Disks, t)
			if err != nil {
				return fmt.Errorf("attach image: %w", err)
			}

			st.image = &img

			return nil
		}},
		{Name: stepLoad, Reason: model.ReasonLoadFromFilesystemFailed, Run: func(ctx context.Context) error {
			if st.object != nil {
				return nil
			}

			obj, err := deps.Load(ctx, t.Kind, *st.image)
			if err == nil && obj == nil {
				err = errors.New("the loader returned no package")
			}

			if err != nil {
				return fmt.Errorf("load package: %w", err)
			}

			st.object = obj

			return nil
		}},
	}
}

// attach: an embedded image is the image directory itself.
func attach(ctx context.Context, disks Disks, t target) (model.Image, error) {
	img := model.Image{
		Kind:     t.Kind,
		Name:     t.Name,
		Version:  t.Version,
		Source:   t.Source,
		Embedded: t.Embedded,
	}

	if t.Embedded {
		img.Path = filepath.Join(disks.EmbeddedDir, t.Name)

		return img, nil
	}

	d, err := diskFor(disks, t.Kind)
	if err != nil {
		return model.Image{}, err
	}

	img.Instance = instance(t.Name, t.Version)
	img.Path = disk.AttachPath(d.Dir, img.Instance)

	if err := d.Backend.Attach(ctx, t.Source.Repository, img.Instance, t.Source.Package, t.Version); err != nil {
		return model.Image{}, err
	}

	return img, nil
}

// detachImage keeps the stored image. Only the image's owner calls it.
func detachImage(ctx context.Context, disks Disks, img model.Image) error {
	if img.Embedded || img.Instance == "" {
		return nil
	}

	d, err := diskFor(disks, img.Kind)
	if err != nil {
		return err
	}

	return d.Backend.Detach(ctx, img.Instance)
}

func diskFor(disks Disks, kind model.Kind) (Disk, error) {
	switch kind {
	case model.KindApplication:
		return disks.Apps, nil
	case model.KindModule:
		return disks.Modules, nil
	default:
		return Disk{}, fmt.Errorf("no disk for package kind %q", kind)
	}
}

// candidate is the preparation of the desired revision; the package Reconciler decides every retry,
// as Current does for its operations.
type candidate struct {
	target target
	// op is the attempt in flight; nil while ready or waiting for a retry.
	op *operation.Operation
	// prepared is the progress; a retry continues after its last successful step.
	prepared prepared
	// failure is the last attempt's failed step with its reason; zero once an attempt succeeded.
	failure gate.Prepared
	// backoff holds the retry, keyed by the target version; a new target is a new candidate, so it
	// starts from the minimum.
	backoff retry.Backoff
	// settings memoizes the final gate's settings check of this candidate; it goes with it.
	settings settingsCheck
}

// outcome is what the gates read: Ready once loaded with no attempt in flight, else the last
// failure, zero before the first one. prepared is read only while no attempt writes it.
func (c *candidate) outcome() gate.Prepared {
	if c.op == nil && c.prepared.object != nil {
		return gate.Prepared{Ready: true, Object: c.prepared.object}
	}

	return c.failure
}

func (r *Reconciler) targetOf(desired model.Desired) target {
	return target{
		Kind:     r.kind,
		Name:     r.name,
		Version:  desired.Version,
		Source:   desired.Source,
		Embedded: desired.Embedded,
	}
}

func (r *Reconciler) preparing(desired model.Desired) bool {
	return r.candidate != nil && r.candidate.target == r.targetOf(desired)
}

// prepare drives the preparation of desired: another revision's candidate is released, a new one
// starts at once, a failed one is retried once its backoff allows.
func (r *Reconciler) prepare(ctx context.Context, desired model.Desired) {
	if !r.preparing(desired) {
		r.stopCandidate(ctx)
		r.candidate = &candidate{target: r.targetOf(desired), settings: settingsCheck{clock: r.clock}}
	}

	c := r.candidate
	if c.op != nil || c.outcome().Ready || c.backoff.Holds(c.target.Version, r.clock.Now()) {
		return
	}

	c.op = operation.Start(ctx, operation.Prepare, c.target.Version, prepareSteps(r.deps, c.target, &c.prepared))
}

// preparationDone is the Done channel of the attempt in flight; nil, which blocks, when none is.
func (r *Reconciler) preparationDone() <-chan operation.Result {
	if r.candidate == nil {
		return nil
	}

	return r.candidate.op.Done()
}

// preparationFinished takes an attempt's result into the candidate. A cancelled attempt is no
// failure: the one who cancelled it decides again.
func (r *Reconciler) preparationFinished(res operation.Result) {
	c := r.candidate
	c.op = nil

	if res.Failed == nil {
		c.backoff.Succeeded()
		c.failure = gate.Prepared{}

		return
	}

	if res.Canceled {
		return
	}

	c.failure = gate.Prepared{Step: res.Failed.Name, Reason: res.Failed.Reason, Message: res.Err.Error()}

	now := r.clock.Now()
	until := c.backoff.Failed(c.target.Version, now)

	r.logger.Warn("prepare revision failed", slog.String("version", c.target.Version),
		slog.String("step", res.Failed.Name), slog.Duration("retry_in", until.Sub(now)), log.Err(res.Err))
}

// takeCandidate cancels and waits for the attempt in flight; the caller owns what it returns. The
// settings memo goes with the candidate.
func (r *Reconciler) takeCandidate() prepared {
	c := r.candidate
	if c.op != nil {
		c.op.Cancel()
		<-c.op.Done()
	}

	r.candidate = nil

	return c.prepared
}

// stopCandidate writes no report, so shutdown can use it.
func (r *Reconciler) stopCandidate(ctx context.Context) {
	if r.candidate != nil {
		r.releaseCandidate(ctx, r.takeCandidate())
	}
}

// releaseCandidate never touches the running image: a Candidate never shares Current's instance
// (gate.Decide).
func (r *Reconciler) releaseCandidate(ctx context.Context, prepared prepared) {
	if prepared.image != nil {
		r.detach(ctx, *prepared.image)
	}
}
