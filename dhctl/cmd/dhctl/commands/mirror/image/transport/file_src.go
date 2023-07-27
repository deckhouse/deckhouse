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
	"os"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
)

type fileImageSource struct {
	ref fileReference
	types.ImageSource
}

// newImageSource returns an ImageSource reading from an existing directory.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ctx context.Context, sys *types.SystemContext, ref fileReference) (types.ImageSource, error) {
	if err := util.ExtractTarGz(ref.StringWithinTransport()); err != nil {
		return nil, err
	}

	dirSrc, err := ref.ImageReference.NewImageSource(ctx, sys)
	if err != nil {
		return nil, err
	}

	return &fileImageSource{ref: ref, ImageSource: dirSrc}, nil
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims). This can be used e.g. to determine which public keys are trusted for this image.
func (s *fileImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *fileImageSource) Close() error {
	if err := os.RemoveAll(s.ref.ImageReference.StringWithinTransport()); err != nil {
		return err
	}
	return s.ImageSource.Close()
}
