/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tagger

import (
	"context"

	"github.com/ovirt/go-ovirt-client/v3"
)

type Tagger interface {
	InitTags(ctx context.Context, tags []string) error
	TagVM(ctx context.Context, vmid ovirtclient.VMID) error
}
