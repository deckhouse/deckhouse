// Copyright 2024 Flant JSC
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

package mirror

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	libmirrorBundle "github.com/deckhouse/deckhouse-cli/pkg/libmirror/bundle"
	libmirrorCtx "github.com/deckhouse/deckhouse-cli/pkg/libmirror/contexts"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	imgBundleExt          = ".tar"
	imgBundleUnpackFormat = "02-01-2006_15-04-05"
)

var (
	imgBundleUnpackMu   sync.Mutex
	imgBundleUnpackInfo map[string]imageBundleUnpackInfo
	isValidationNeeded  = true
)

func init() {
	imgBundleUnpackInfo = map[string]imageBundleUnpackInfo{}
}

type imageBundleUnpackInfo struct {
	path string
	err  error
}

func UnpackAndValidateImgBundle(ctx context.Context, imgBundlePath string) (string, error) {
	logger := Logger{}

	imgBundleUnpackMu.Lock()
	defer imgBundleUnpackMu.Unlock()

	if unpackInfo, ok := imgBundleUnpackInfo[imgBundlePath]; ok {
		logger.InfoLn("Using bundle at", unpackInfo.path)
		return unpackInfo.path, unpackInfo.err
	}

	unpackPath, unpackErr := unpackAndValidateImgBundle(ctx, imgBundlePath)
	imgBundleUnpackInfo[imgBundlePath] = imageBundleUnpackInfo{
		path: unpackPath,
		err:  unpackErr,
	}
	logger.InfoLn("Using bundle at", unpackPath)
	return unpackPath, unpackErr
}

func unpackAndValidateImgBundle(ctx context.Context, imgBundlePath string) (string, error) {
	logger := Logger{}

	unpackPath, err := unpackImgBundle(ctx, imgBundlePath)
	if err != nil {
		return unpackPath, fmt.Errorf("failed to unpack img bundle: %w", err)
	}

	if isValidationNeeded {
		if err := libmirrorBundle.ValidateUnpackedBundle(
			&libmirrorCtx.PushContext{
				BaseContext: libmirrorCtx.BaseContext{
					UnpackedImagesPath: unpackPath,
					Logger:             &logger,
				},
			},
		); err != nil {
			return unpackPath, fmt.Errorf("invalid bundle: %w", err)
		}
	} else {
		log.DebugLn("Bundle validation is disabled by build tag")
	}

	return unpackPath, nil
}

func unpackImgBundle(ctx context.Context, imgBundlePath string) (string, error) {
	logger := Logger{}

	if filepath.Ext(imgBundlePath) != imgBundleExt {
		return imgBundlePath, nil
	}

	unpackedImagesPath := filepath.Join(app.TmpDirName, "img_bundles", time.Now().Format(imgBundleUnpackFormat))

	err := logger.Process("Unpacking Deckhouse bundle", func() error {
		return libmirrorBundle.UnpackContext(
			ctx,
			&libmirrorCtx.BaseContext{
				BundlePath:         imgBundlePath,
				UnpackedImagesPath: unpackedImagesPath,
				Logger:             &logger,
			},
		)
	})

	if err != nil {
		return unpackedImagesPath, err
	}

	tomb.RegisterOnShutdown(
		fmt.Sprintf("Delete untar images bundle %s", unpackedImagesPath),
		func() { os.RemoveAll(unpackedImagesPath) },
	)
	return unpackedImagesPath, nil
}
