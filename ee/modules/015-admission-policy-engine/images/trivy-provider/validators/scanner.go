/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validators

import (
	"context"
	"net/http"

	"github.com/aquasecurity/trivy/pkg/cache"
	fartifact "github.com/aquasecurity/trivy/pkg/fanal/artifact"
	fimage "github.com/aquasecurity/trivy/pkg/fanal/artifact/image"
	"github.com/aquasecurity/trivy/pkg/fanal/image"
	ftypes "github.com/aquasecurity/trivy/pkg/fanal/types"
	"github.com/aquasecurity/trivy/pkg/rpc/client"
	"github.com/aquasecurity/trivy/pkg/scanner"
	"github.com/aquasecurity/trivy/pkg/types"

	_ "modernc.org/sqlite"
)

func scanArtifact(ctx context.Context, imageName, remoteURL string, customHeaders http.Header, scanOpts types.ScanOptions) (types.Report, error) {
	img, cleanup, err := image.NewContainerImage(ctx, imageName, ftypes.ImageOptions{
		ImageSources: ftypes.ImageSources{ftypes.RemoteImageSource},
	})
	if err != nil {
		return types.Report{}, err
	}
	defer cleanup()

	artifactCache := cache.NewRemoteCache(remoteURL, customHeaders, false)
	artifact, err := fimage.NewArtifact(img, artifactCache, fartifact.Option{DisabledHandlers: []ftypes.HandlerType{ftypes.UnpackagedPostHandler}})
	if err != nil {
		return types.Report{}, err
	}

	clientScanner := client.NewScanner(client.ScannerOption{RemoteURL: remoteURL, CustomHeaders: customHeaders})
	myScanner := scanner.NewScanner(clientScanner, artifact)

	return myScanner.ScanArtifact(ctx, scanOpts)
}
