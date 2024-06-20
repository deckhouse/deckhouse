/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

func CreateBundle(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec, params *InputParams) (*FilesBundle, error) {
	bundle := FilesBundle{}

	for _, cert := range manifestsSpec.GeneratedCertificates {
		certBundle, err := CreateCertBundle(ctx, &cert)
		if err != nil {
			return nil, err
		}
		bundle.Certs = append(bundle.Certs, *certBundle)
	}

	renderData, err := pkg_cfg.GetDataForManifestRendering(pkg_cfg.NewExtraDataForManifestRendering(params.StaticPods.MasterPeers))
	if err != nil {
		return nil, err
	}

	for _, manifest := range manifestsSpec.Manifests {
		manifestBundle, err := CreateManifestBundle(ctx, &manifest, &renderData)
		if err != nil {
			return nil, err
		}
		bundle.Manifests = append(bundle.Manifests, *manifestBundle)
	}

	for _, staticPod := range manifestsSpec.StaticPods {
		staticPodBundle, err := CreateStaticPodBundle(ctx, &staticPod, &renderData)
		if err != nil {
			return nil, err
		}
		bundle.StaticPods = append(bundle.StaticPods, *staticPodBundle)
	}
	return &bundle, nil
}

func CheckDest(ctx context.Context, bundle *FilesBundle, params *InputParams) error {
	for i, cert := range bundle.Certs {
		err := CheckCertDest(ctx, &cert, params)
		bundle.Certs[i] = cert
		if err != nil {
			return err
		}
	}

	for i, manifest := range bundle.Manifests {
		err := CheckManifestDest(ctx, &manifest, params)
		bundle.Manifests[i] = manifest
		if err != nil {
			return err
		}
	}

	for i, staticPod := range bundle.StaticPods {
		err := CheckStaticPodDest(ctx, &staticPod, params)
		bundle.StaticPods[i] = staticPod
		if err != nil {
			return err
		}
	}
	return nil
}

func Update(ctx context.Context, bundle *FilesBundle) error {
	for i, cert := range bundle.Certs {
		err := UpdateCertDest(ctx, &cert)
		bundle.Certs[i] = cert
		if err != nil {
			return err
		}
	}

	for i, manifest := range bundle.Manifests {
		err := UpdateManifestDest(ctx, &manifest)
		bundle.Manifests[i] = manifest
		if err != nil {
			return err
		}
	}

	for i, staticPod := range bundle.StaticPods {
		err := UpdateStaticPodDest(ctx, &staticPod)
		bundle.StaticPods[i] = staticPod
		if err != nil {
			return err
		}
	}
	return nil
}

func PatchStaticPodsDestForRestart(ctx context.Context, bundle *FilesBundle) error {
	for i, staticPod := range bundle.StaticPods {
		err := PatchStaticPodDestForRestart(ctx, bundle, &staticPod)
		bundle.StaticPods[i] = staticPod
		if err != nil {
			return err
		}
	}
	return nil
}

func Delete(ctx context.Context, manifestsSpec *pkg_cfg.ManifestsSpec) error {
	for _, cert := range manifestsSpec.GeneratedCertificates {
		err := DeleteCertDest(ctx, &cert)
		if err != nil {
			return err
		}
	}

	for _, manifest := range manifestsSpec.Manifests {
		err := DeleteManifestDest(ctx, &manifest)
		if err != nil {
			return err
		}
	}

	for _, staticPod := range manifestsSpec.StaticPods {
		err := DeleteStaticPodDest(ctx, &staticPod)
		if err != nil {
			return err
		}
	}
	return nil
}
