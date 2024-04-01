/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tagger

import (
	"context"
	"fmt"

	ovirt "github.com/ovirt/go-ovirt-client/v3"
)

type TaggerImpl struct {
	cl     ovirt.Client
	tagIDs []ovirt.TagID
}

func NewTagger(client ovirt.Client) *TaggerImpl {
	return &TaggerImpl{
		cl:     client,
		tagIDs: make([]ovirt.TagID, 0),
	}
}

func (t *TaggerImpl) InitTags(ctx context.Context, tags []string) error {
	tagsToCreate := make(map[string]struct{})
	for _, tag := range tags {
		tagsToCreate[tag] = struct{}{}
	}

	existingTags, err := t.cl.ListTags()
	if err != nil {
		return fmt.Errorf("Read existing tags from zVirt: %w", err)
	}

	for _, existingTag := range existingTags {
		if _, found := tagsToCreate[existingTag.Name()]; found {
			delete(tagsToCreate, existingTag.Name())
		}
	}

	ctp := ovirt.NewCreateTagParams().MustWithDescription("Tag created by cluster-api-provider-zvirt, do not delete")
	for key, val := range tagsToCreate {
		tagName := fmt.Sprintf("%s-%s", key, val)
		tag, err := t.cl.WithContext(ctx).CreateTag(tagName, ctp)
		if err != nil {
			return fmt.Errorf("Create %s tag: %w", tagName, err)
		}
		t.tagIDs = append(t.tagIDs, tag.ID())
	}

	return nil
}

func (t *TaggerImpl) TagVM(ctx context.Context, vmid ovirt.VMID) error {
	cl := t.cl.WithContext(ctx)
	for _, tagID := range t.tagIDs {
		if err := cl.AddTagToVM(vmid, tagID); err != nil {
			return fmt.Errorf("Tag VM[id = %s] with Tag[id = %s]: %w", err)
		}
	}
	return nil
}
