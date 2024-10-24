/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package steps

import (
	"context"
	"fmt"
	"os"
	"strings"

	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_files "system-registry-manager/pkg/files"
	pkg_k8s_manifests "system-registry-manager/pkg/kubernetes/manifests"
	pkg_utils "system-registry-manager/pkg/utils"
)

const (
	certsCheckSumAnnotation      = "certschecksum"
	manifestsCheckSumAnnotation  = "manifestschecksum"
	staticPodsCheckSumAnnotation = "staticpodschecksum"
	masterPeersLineContent       = "master.peers"
	isRaftBootstrapLineContent   = "master.raftBootstrap"
)

// CreateStaticPodBundle reads the static pod manifest template, renders it with the provided data, and returns a StaticPodBundle.
func CreateStaticPodBundle(ctx context.Context, staticPodManifestSpec *pkg_cfg.StaticPodManifestSpec, renderData *map[string]interface{}) (*StaticPodBundle, error) {
	fileContent, err := os.ReadFile(staticPodManifestSpec.InputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading static pod manifest template: %v", err)
	}

	fileRenderContent, err := pkg_files.RenderTemplate(string(fileContent), *renderData)
	if err != nil {
		return nil, fmt.Errorf("error rendering static pod manifest template: %v", err)
	}

	return &StaticPodBundle{
		File: FileBundle{
			DestPath: staticPodManifestSpec.DestPath,
			Content:  fileRenderContent,
		},
		Check: FileCheck{},
	}, nil
}

// CheckStaticPodDest checks if the static pod destination needs to be created or updated.
func CheckStaticPodDest(ctx context.Context, staticPodBundle *StaticPodBundle, params *InputParams) error {
	if !params.StaticPods.UpdateOrCreate {
		return nil
	}

	if !pkg_files.IsPathExists(staticPodBundle.File.DestPath) {
		staticPodBundle.Check.NeedCreate = true
		return nil
	}

	destFileContent, err := os.ReadFile(staticPodBundle.File.DestPath)
	if err != nil {
		return fmt.Errorf("error reading static pod manifest: %v", err)
	}

	preparedSourceFileContent, err := prepareStaticPodsBeforeCompare(staticPodBundle.File.Content, params)
	if err != nil {
		return fmt.Errorf("error preparing source static pod manifest for comparison: %v", err)
	}

	preparedDestFileContent, err := prepareStaticPodsBeforeCompare(string(destFileContent), params)
	if err != nil {
		return fmt.Errorf("error preparing destination static pod manifest for comparison: %v", err)
	}
	if pkg_utils.EqualYaml([]byte(preparedSourceFileContent), []byte(preparedDestFileContent)) {
		staticPodBundle.Check.NeedUpdate = false
		return nil
	}

	checkSumEq, err := pkg_files.CompareChecksumByFileContent(preparedSourceFileContent, preparedDestFileContent)
	if err != nil {
		return fmt.Errorf("error comparing checksums for file %s: %v", staticPodBundle.File.DestPath, err)
	}
	staticPodBundle.Check.NeedUpdate = !checkSumEq
	return nil
}

// UpdateStaticPodDest writes the static pod manifest to the destination if it needs to be created or updated.
func UpdateStaticPodDest(ctx context.Context, staticPodBundle *StaticPodBundle) error {
	if staticPodBundle.Check.NeedCreateOrUpdate() {
		if err := pkg_files.WriteFile(staticPodBundle.File.DestPath, []byte(staticPodBundle.File.Content), pkg_cfg.DefaultFileMode); err != nil {
			return fmt.Errorf("error writing static pod manifest to %s: %v", staticPodBundle.File.DestPath, err)
		}
	}
	return nil
}

// PatchStaticPodDestForRestart patches the static pod manifest with new checksums if needed.
func PatchStaticPodDestForRestart(ctx context.Context, filesBundle *FilesBundle, staticPodBundle *StaticPodBundle) error {
	needChangeCerts := false
	needChangeManifests := false
	needChangeStaticPods := false

	for _, cert := range filesBundle.Certs {
		if cert.Check.NeedCreateOrUpdate() {
			needChangeCerts = true
		}
	}

	for _, manifest := range filesBundle.Manifests {
		if manifest.Check.NeedCreateOrUpdate() {
			needChangeManifests = true
		}
	}

	for _, staticPod := range filesBundle.StaticPods {
		if staticPod.Check.NeedCreateOrUpdate() {
			needChangeStaticPods = true
		}
	}

	annotations := map[string]string{}
	if needChangeCerts {
		annotations[certsCheckSumAnnotation] = pkg_utils.GenerateHash()
	}
	if needChangeManifests {
		annotations[manifestsCheckSumAnnotation] = pkg_utils.GenerateHash()
	}
	if needChangeStaticPods {
		annotations[staticPodsCheckSumAnnotation] = pkg_utils.GenerateHash()
	}

	if len(annotations) != 0 {
		content, err := os.ReadFile(staticPodBundle.File.DestPath)
		if err != nil {
			return fmt.Errorf("error reading static pod manifest: %v", err)
		}

		newContent, err := pkg_k8s_manifests.ChangePodAnnotations(content, annotations)
		if err != nil {
			return fmt.Errorf("error changing pod annotations to static pod manifest: %v", err)
		}

		if err := pkg_files.WriteFile(staticPodBundle.File.DestPath, []byte(newContent), pkg_cfg.DefaultFileMode); err != nil {
			return fmt.Errorf("error writing static pod manifest to %s: %v", staticPodBundle.File.DestPath, err)
		}
	}
	return nil
}

// DeleteStaticPodDest deletes the static pod manifest file from the destination.
func DeleteStaticPodDest(ctx context.Context, staticPodManifestSpec *pkg_cfg.StaticPodManifestSpec) error {
	if err := pkg_files.DeleteFileIfExist(staticPodManifestSpec.DestPath); err != nil {
		return fmt.Errorf("error deleting static pod from '%s': %w", staticPodManifestSpec.DestPath, err)
	}
	return nil
}

// prepareStaticPodsBeforeCompare prepares the static pod manifest content for comparison by removing specific lines and annotations.
func prepareStaticPodsBeforeCompare(content string, params *InputParams) (string, error) {
	if !params.StaticPods.Check.WithMasterPeers {
		content = removeLineByParams(content, []string{masterPeersLineContent})
	}
	if !params.StaticPods.Check.WithIsRaftBootstrap {
		content = removeLineByParams(content, []string{isRaftBootstrapLineContent})
	}

	annotations := map[string]string{
		certsCheckSumAnnotation:      "",
		manifestsCheckSumAnnotation:  "",
		staticPodsCheckSumAnnotation: "",
	}
	newContent, err := pkg_k8s_manifests.ChangePodAnnotations([]byte(content), annotations)
	if err != nil {
		return "", fmt.Errorf("error changing pod annotations for static pod manifest: %v", err)
	}
	return string(newContent), nil
}

// removeLineByParams removes lines from the manifest that contain any of the specified parameters.
func removeLineByParams(manifest string, params []string) string {
	lines := strings.Split(manifest, "\n")
	var newLines []string

	for _, line := range lines {
		include := true
		for _, param := range params {
			if strings.Contains(line, param) {
				include = false
				break
			}
		}
		if include {
			newLines = append(newLines, line)
		}
	}
	return strings.Join(newLines, "\n")
}
