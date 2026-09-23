/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tagger

import (
	"context"
	"fmt"
	"sync"

	ovirt "github.com/ovirt/go-ovirt-client/v3"
)

type TaggerImpl struct {
	cl ovirt.Client

	// lock guards tagIDs. The tags are initialized by a runnable of the manager, which runs
	// concurrently with the reconcile loops that tag the VMs.
	lock   sync.Mutex
	tagIDs []ovirt.TagID
}

func NewTagger(client ovirt.Client) *TaggerImpl {
	return &TaggerImpl{
		cl:     client,
		tagIDs: make([]ovirt.TagID, 0),
	}
}

func (t *TaggerImpl) InitTags(ctx context.Context, tags []string) error {
	cl := t.cl.WithContext(ctx)

	tagsToCreate := make(map[string]struct{})
	for _, tag := range tags {
		tagsToCreate[tag] = struct{}{}
	}

	existingTags, err := cl.ListTags()
	if err != nil {
		return fmt.Errorf("Read existing tags from zVirt: %w", err)
	}

	for _, existingTag := range existingTags {
		delete(tagsToCreate, existingTag.Name())
	}

	ctp := ovirt.NewCreateTagParams().MustWithDescription("Tag created by cluster-api-provider-zvirt, do not delete")
	for tagName := range tagsToCreate {
		tag, err := cl.CreateTag(tagName, ctp)
		if err != nil {
			return fmt.Errorf("Create %s tag: %w", tagName, err)
		}

		// Published one by one, so that the tags that could be created are applied to the
		// VMs even when a later one fails.
		t.lock.Lock()
		t.tagIDs = append(t.tagIDs, tag.ID())
		t.lock.Unlock()
	}

	return nil
}

func (t *TaggerImpl) TagVM(ctx context.Context, vmID ovirt.VMID) error {
	t.lock.Lock()
	// InitTags only appends, so the elements up to this length are never rewritten.
	tagIDs := t.tagIDs
	t.lock.Unlock()

	cl := t.cl.WithContext(ctx)
	for _, tagID := range tagIDs {
		if err := cl.AddTagToVM(vmID, tagID); err != nil {
			return fmt.Errorf("Tag VM[id = %s] with Tag[id = %s]: %w", vmID, tagID, err)
		}
	}
	return nil
}
