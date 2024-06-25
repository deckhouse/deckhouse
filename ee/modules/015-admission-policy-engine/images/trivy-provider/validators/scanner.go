/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validators

import (
	"context"
	"github.com/google/go-containerregistry/pkg/name"
	"net/http"
	"os"

	"github.com/aquasecurity/trivy/pkg/cache"
	"github.com/aquasecurity/trivy/pkg/fanal/artifact"
	fimage "github.com/aquasecurity/trivy/pkg/fanal/artifact/image"
	"github.com/aquasecurity/trivy/pkg/fanal/image"
	ftypes "github.com/aquasecurity/trivy/pkg/fanal/types"
	"github.com/aquasecurity/trivy/pkg/javadb"
	"github.com/aquasecurity/trivy/pkg/rpc/client"
	"github.com/aquasecurity/trivy/pkg/scanner"
	"github.com/aquasecurity/trivy/pkg/types"

	_ "modernc.org/sqlite"
)

var inited bool

func scanArtifact(ctx context.Context, imageName, remoteURL string, customHeaders http.Header, scanOpts types.ScanOptions) (types.Report, error) {
	if !inited {
		javadbImage := os.Getenv("TRIVY_JAVA_DB_IMAGE")
		if len(javadbImage) == 0 {
			javadbImage = "ghcr.io/aquasecurity/trivy-java-db:1"
		}

		ref, err := name.ParseReference(javadbImage)
		if err != nil {
			return types.Report{}, err
		}

		javadb.Init("/home/javadb", ref, false, true, ftypes.RegistryOptions{Insecure: false})
		inited = true
		javadb.Update()
		if err != nil {
			return types.Report{}, err
		}
	}

	img, cleanup, err := image.NewContainerImage(ctx, imageName, ftypes.ImageOptions{
		ImageSources: ftypes.ImageSources{ftypes.RemoteImageSource},
	})
	if err != nil {
		return types.Report{}, err
	}
	defer cleanup()

	artifactCache := cache.NewRemoteCache(remoteURL, customHeaders, false)
	artf, err := fimage.NewArtifact(img, artifactCache, artifact.Option{DisabledHandlers: []ftypes.HandlerType{ftypes.UnpackagedPostHandler}})
	if err != nil {
		return types.Report{}, err
	}

	clientScanner := client.NewScanner(client.ScannerOption{RemoteURL: remoteURL, CustomHeaders: customHeaders})
	myScanner := scanner.NewScanner(clientScanner, artf)
	return myScanner.ScanArtifact(ctx, scanOpts)
}
