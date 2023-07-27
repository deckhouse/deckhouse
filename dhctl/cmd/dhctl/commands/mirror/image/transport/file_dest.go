// Copyright 2023 Flant JSC
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

package transport

import (
	"context"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
)

type fileImageDestination struct {
	ref fileReference
	types.ImageDestination
}

// newImageDestination returns an ImageDestination for writing to a directory.
func newImageDestination(ctx context.Context, sys *types.SystemContext, ref fileReference) (types.ImageDestination, error) {
	dirDest, err := ref.ImageReference.NewImageDestination(ctx, sys)
	if err != nil {
		return nil, err
	}
	return &fileImageDestination{ref: ref, ImageDestination: dirDest}, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *fileImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *fileImageDestination) Close() error {
	return d.ImageDestination.Close()
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *fileImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	if err := util.CompressDir(util.TrimTarGzExt(d.ref.StringWithinTransport()), true); err != nil {
		return err
	}
	return d.ImageDestination.Commit(ctx, unparsedToplevel)
}
