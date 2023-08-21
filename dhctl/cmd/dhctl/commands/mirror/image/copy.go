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

package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func CopyImage(ctx context.Context, src, dest *ImageConfig, policyContext *signature.PolicyContext, logger log.Logger, opts ...CopyOption) (bool, error) {
	copyOptions := &copyOptions{copyOptions: &copy.Options{}}
	opts = append(opts, withSourceAuth(src.AuthConfig()), withDestAuth(dest.AuthConfig()))
	for _, opt := range opts {
		opt(copyOptions)
	}

	if err := checkImageExists(ctx, src, dest, copyOptions); err == nil {
		return true, nil
	} else {
		logger.LogDebugF("No image in dest registry equal to source image: %w\n", err)
	}

	srcRef, err := src.imageReference(true, copyOptions.dryRun)
	if err != nil {
		return false, err
	}
	defer src.close()

	destRef, err := dest.imageReference(false, copyOptions.dryRun)
	if err != nil {
		return false, err
	}

	if writer := copyOptions.copyOptions.ReportWriter; writer != nil {
		msg := fmt.Sprintf("\nCopying %s image to %s...\n", trimRef(srcRef), trimRef(destRef))
		if _, err := writer.Write([]byte(msg)); err != nil {
			return false, err
		}
	}

	if copyOptions.dryRun {
		return false, nil
	}

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, copyOptions.copyOptions)
	return false, err
}

func NewPolicyContext() (*signature.PolicyContext, error) {
	// https://github.com/containers/skopeo/blob/v1.12.0/cmd/skopeo/main.go#L141
	return signature.NewPolicyContext(&signature.Policy{
		Default: signature.PolicyRequirements{signature.NewPRInsecureAcceptAnything()},
	})
}

func trimRef(ref types.ImageReference) string {
	return strings.TrimLeft(ref.StringWithinTransport(), "/")
}

func checkImageExists(ctx context.Context, sourceImg, destImg *ImageConfig, copyOpt *copyOptions) error {
	if destImg.RegistryTransport() == fileTransport {
		return fmt.Errorf("image existence not implemented in file registry")
	}

	if digest := sourceImg.Digest(); digest != "" {
		destImg = destImg.WithDigest(digest).WithTag("")
	} else {
		destImg = destImg.WithTag(sourceImg.Tag()).WithDigest("")
	}

	destImgRef, err := destImg.imageReference(false, copyOpt.dryRun)
	if err != nil {
		return err
	}

	destImgSource, err := destImgRef.NewImageSource(ctx, copyOpt.copyOptions.DestinationCtx)
	if err != nil {
		return err
	}

	destManifest, _, err := destImgSource.GetManifest(ctx, nil)
	if err != nil {
		return err
	}

	sourceImgRef, err := sourceImg.imageReference(true, copyOpt.dryRun)
	if err != nil {
		return err
	}

	sourceImgSource, err := sourceImgRef.NewImageSource(ctx, copyOpt.copyOptions.SourceCtx)
	if err != nil {
		return err
	}

	sourceManifest, _, err := sourceImgSource.GetManifest(ctx, nil)
	if err != nil {
		return err
	}

	if string(sourceManifest) != string(destManifest) {
		return fmt.Errorf("images are not equal for %s and %s", sourceImgRef.StringWithinTransport(), destImgRef.StringWithinTransport())
	}
	return nil
}
