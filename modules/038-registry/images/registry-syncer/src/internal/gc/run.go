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

package gc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Registry is the store to collect, as this replica reaches its own.
type Registry struct {
	// Address is where this replica's registry answers. The loopback address: a replica
	// collects its own store and nobody else's.
	Address string

	// Insecure allows plain HTTP, for a store served without TLS.
	Insecure bool

	// Scope limits which repositories are considered, so that a store holding anything
	// beyond this module's images is not judged by this module's rules.
	Scope string

	// Options are how to authenticate and whom to trust.
	Options []remote.Option
}

// Collector reclaims the disk of one replica.
type Collector struct {
	Log *slog.Logger

	// Registry is this replica's own store.
	Registry Registry

	// Releases is what the cluster is running, which decides what may go.
	Releases Releases

	// Sweep reclaims the blobs left unreachable by the deleted manifests. Optional: with
	// no sweeper the manifests are deleted and the disk is not reclaimed, which is
	// useless but not harmful.
	Sweep Sweeper

	// DryRun decides what would go without touching anything.
	DryRun bool
}

// Sweeper reclaims unreachable blobs.
type Sweeper interface {
	Sweep(ctx context.Context) error
}

// Report is what a run did.
type Report struct {
	// Considered is how many tags were examined.
	Considered int

	// Deleted are the references that were removed.
	Deleted []string

	// Kept counts the tags left alone, by the reason they were kept. The interesting
	// number when a disk is still full after a run.
	Kept map[string]int

	// Failed are the references that could not be removed, which is not a failure of the
	// run: the disk is simply less reclaimed than it could have been.
	Failed []string

	// Swept reports whether blob reclamation ran.
	Swept bool
}

// Run collects the store.
//
// Deletes manifests first and reclaims blobs second, because that is the only order that
// works: distribution's collector reclaims what no manifest references, so the references
// have to be gone before it runs.
//
// A failure to delete one manifest does not stop the run. The blobs of the ones that were
// deleted are still reclaimable, and a store that is partly collected is the ordinary
// outcome on a registry that is also serving.
func (c *Collector) Run(ctx context.Context) (Report, error) {
	report := Report{Kept: map[string]int{}}

	keep, err := c.Releases.Keep()
	if err != nil {
		// Refused rather than attempted. Every deletion here is justified by a comparison
		// against the deployed release, and without one there is no justification for any
		// of them.
		return report, fmt.Errorf("cannot decide what to keep: %w", err)
	}

	tags, err := c.tags(ctx)
	if err != nil {
		return report, err
	}
	report.Considered = len(tags)

	var doomed []name.Tag
	for _, tag := range tags {
		decision := Judge(tag.TagStr(), keep, c.Releases.Deployed)
		if !decision.Delete {
			report.Kept[decision.Reason]++
			continue
		}
		doomed = append(doomed, tag)
	}

	if len(doomed) == 0 {
		c.Log.Info("nothing to reclaim", "considered", report.Considered, "kept", report.Kept)
		return report, nil
	}

	if c.DryRun {
		for _, tag := range doomed {
			report.Deleted = append(report.Deleted, tag.String())
		}
		c.Log.Info("dry run, nothing was touched",
			"would_delete", len(report.Deleted), "kept", report.Kept)
		return report, nil
	}

	for _, tag := range doomed {
		if err := remote.Delete(tag, c.Registry.Options...); err != nil {
			// One reference that would not go is not a reason to abandon the rest, nor to
			// skip the sweep: what did get deleted is still reclaimable.
			c.Log.Warn("cannot delete a reference", "reference", tag.String(), "error", err.Error())
			report.Failed = append(report.Failed, tag.String())
			continue
		}
		report.Deleted = append(report.Deleted, tag.String())
	}

	if c.Sweep == nil || len(report.Deleted) == 0 {
		return report, nil
	}

	if err := c.Sweep.Sweep(ctx); err != nil {
		// The manifests are gone either way, so the run did something. Reported so that a
		// disk that never shrinks is attributable to the sweep rather than to the
		// judgement.
		return report, fmt.Errorf("reclaiming the blobs of %d deleted references: %w", len(report.Deleted), err)
	}
	report.Swept = true

	c.Log.Info("reclaimed the store",
		"deleted", len(report.Deleted), "failed", len(report.Failed), "kept", report.Kept)
	return report, nil
}

// tags lists every tag of every in-scope repository of this replica's own store.
func (c *Collector) tags(ctx context.Context) ([]name.Tag, error) {
	puller, err := remote.NewPuller(c.Registry.Options...)
	if err != nil {
		return nil, fmt.Errorf("building the client: %w", err)
	}

	var options []name.Option
	if c.Registry.Insecure {
		options = append(options, name.Insecure)
	}
	registry, err := name.NewRegistry(c.Registry.Address, options...)
	if err != nil {
		return nil, fmt.Errorf("parsing the registry address %q: %w", c.Registry.Address, err)
	}

	catalogger, err := puller.Catalogger(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("listing the repositories: %w", err)
	}

	var tags []name.Tag
	for catalogger.HasNext() {
		page, err := catalogger.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading the next page of repositories: %w", err)
		}

		for _, repository := range page.Repos {
			if !c.inScope(repository) {
				continue
			}

			repositoryName := registry.Repo(repository)
			lister, err := puller.Lister(ctx, repositoryName)
			if err != nil {
				return nil, fmt.Errorf("listing the tags of %s: %w", repositoryName, err)
			}

			for lister.HasNext() {
				page, err := lister.Next(ctx)
				if err != nil {
					return nil, fmt.Errorf("reading the next page of tags of %s: %w", repositoryName, err)
				}
				for _, tag := range page.Tags {
					tags = append(tags, repositoryName.Tag(tag))
				}
			}
		}
	}

	return tags, nil
}

// inScope keeps the collector from judging repositories this module does not own.
//
// A store may hold more than this module put there, and the rules here are about Deckhouse
// releases. Applying them to somebody else's images would delete by a measure that has
// nothing to do with them.
func (c *Collector) inScope(repository string) bool {
	scope := strings.Trim(c.Registry.Scope, "/")
	if scope == "" {
		return true
	}

	trimmed := strings.Trim(repository, "/")
	return trimmed == scope || strings.HasPrefix(trimmed, scope+"/")
}

// Binary reclaims blobs by running the registry's own collector.
//
// The registry binary rather than an implementation of our own, because the store layout is
// distribution's and the cost of misunderstanding it is deleting a blob that is still
// referenced. It is the same binary the serving container runs, imported into this image at
// build time.
type Binary struct {
	Log *slog.Logger

	// Path of the registry binary.
	Path string

	// ConfigPath is the rendered configuration, which is where the collector learns the
	// store's location.
	ConfigPath string
}

// Sweep runs the collector.
//
// `--delete-untagged` is deliberately NOT passed. In a pass-through cache the manifests
// fetched on a cache miss are untagged by nature, and removing them on every run would empty
// the cache of exactly the images it was asked to hold — turning a garbage collection into a
// cache flush. What this run reclaims is what the deleted tags made unreachable.
func (b *Binary) Sweep(ctx context.Context) error {
	if b.Path == "" || b.ConfigPath == "" {
		return errors.New("no registry binary to reclaim blobs with")
	}

	command := exec.CommandContext(ctx, b.Path, "garbage-collect", b.ConfigPath)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s garbage-collect: %w: %s", b.Path, err, strings.TrimSpace(string(output)))
	}

	b.Log.Info("the registry reclaimed its unreachable blobs",
		"output", strings.TrimSpace(string(output)))
	return nil
}
