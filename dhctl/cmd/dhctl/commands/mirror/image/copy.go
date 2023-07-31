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
	"os"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
)

func CopyImage(ctx context.Context, src, dest *ImageConfig, policyContext *signature.PolicyContext, opts ...CopyOption) (bool, error) {
	srcRef, err := src.imageReference()
	if err != nil {
		return false, err
	}

	destRef, err := dest.imageReference()
	if err != nil {
		return false, err
	}

	copyOptions := &copyOptions{copyOptions: &copy.Options{ReportWriter: os.Stdout}}

	opts = append(opts, withSourceAuth(src.AuthConfig()), withDestAuth(dest.AuthConfig()))
	for _, opt := range opts {
		opt(copyOptions)
	}

	if err := checkImageExists(ctx, destRef, copyOptions.copyOptions.DestinationCtx); err == nil {
		return true, nil
	}

	msg := fmt.Sprintf("\nCopying %s image to %s...\n", trimRef(srcRef), trimRef(destRef))
	if _, err := copyOptions.copyOptions.ReportWriter.Write([]byte(msg)); err != nil {
		return false, err
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

func checkImageExists(ctx context.Context, imgRef types.ImageReference, sysCtx *types.SystemContext) error {
	imgSource, err := imgRef.NewImageSource(ctx, sysCtx)
	if err != nil {
		return err
	}

	_, _, err = imgSource.GetManifest(ctx, nil)
	return err
}
