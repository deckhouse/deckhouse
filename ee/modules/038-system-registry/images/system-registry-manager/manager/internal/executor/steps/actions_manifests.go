/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"fmt"
	"os"

	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_files "system-registry-manager/pkg/files"
)

// CreateManifestBundle reads the manifest template, renders it with the provided data, and returns a ManifestBundle.
func CreateManifestBundle(ctx context.Context, manifestSpec *pkg_cfg.ManifestSpec, renderData *map[string]interface{}) (*ManifestBundle, error) {
	fileContent, err := os.ReadFile(manifestSpec.InputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading manifest template: %v", err)
	}

	fileRenderContent, err := pkg_files.RenderTemplate(string(fileContent), *renderData)
	if err != nil {
		return nil, fmt.Errorf("error rendering manifest template: %v", err)
	}

	return &ManifestBundle{
		File: FileBundle{
			DestPath: manifestSpec.DestPath,
			Content:  fileRenderContent,
		},
		Check: FileCheck{},
	}, nil
}

// CheckManifestDest checks if the manifest destination needs to be created or updated.
func CheckManifestDest(ctx context.Context, manifestBundle *ManifestBundle, params *InputParams) error {
	if !params.Manifests.UpdateOrCreate {
		return nil
	}

	if !pkg_files.IsPathExists(manifestBundle.File.DestPath) {
		manifestBundle.Check.NeedCreate = true
		return nil
	}

	checkSumEq, err := pkg_files.CompareChecksumByDestFilePath(manifestBundle.File.Content, manifestBundle.File.DestPath)
	if err != nil {
		return fmt.Errorf("error comparing checksums for file %s: %v", manifestBundle.File.DestPath, err)
	}

	manifestBundle.Check.NeedUpdate = !checkSumEq
	return nil
}

// UpdateManifestDest writes the manifest to the destination if it needs to be created or updated.
func UpdateManifestDest(ctx context.Context, manifestBundle *ManifestBundle) error {
	if manifestBundle.Check.NeedCreateOrUpdate() {
		if err := pkg_files.WriteFile(manifestBundle.File.DestPath, []byte(manifestBundle.File.Content), pkg_cfg.DefaultFileMode); err != nil {
			return fmt.Errorf("error writing manifest to %s: %v", manifestBundle.File.DestPath, err)
		}
	}
	return nil
}

// DeleteManifestDest deletes the manifest file from the destination.
func DeleteManifestDest(ctx context.Context, manifestSpec *pkg_cfg.ManifestSpec) error {
	if err := pkg_files.DeleteFileIfExist(manifestSpec.DestPath); err != nil {
		return fmt.Errorf("error deleting manifest from '%s': %w", manifestSpec.DestPath, err)
	}
	return nil
}
