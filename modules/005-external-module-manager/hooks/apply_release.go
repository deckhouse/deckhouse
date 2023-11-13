/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	deckhouse_config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// check symlinks exist on the startup
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 5,
	},
	Queue: "/modules/external-module-source/apply-release",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleRelease",
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterRelease,
		},
	},
}, applyModuleRelease)

var (
	fsSynchronized = false
)

func applyModuleRelease(input *go_hook.HookInput) error {
	var modulesChangedReason string
	var fsModulesLinks map[string]string

	snap := input.Snapshots["releases"]

	externalModulesDir := os.Getenv("EXTERNAL_MODULES_DIR")
	if externalModulesDir == "" {
		input.LogEntry.Warn("EXTERNAL_MODULES_DIR is not set")
		return nil
	}
	// directory for symlinks will actual versions to all external-modules
	symlinksDir := filepath.Join(externalModulesDir, "modules")
	ts := metav1.NewTime(time.Now().UTC())

	// run only once on startup
	if !fsSynchronized {
		var err error
		fsModulesLinks, err = readModulesFromFS(symlinksDir)
		if err != nil {
			input.LogEntry.Errorf("Could not read modules from fs: %s", err)
			return nil
		}
	}

	moduleReleases := make(map[string][]enqueueRelease, 0)
	for _, sn := range snap {
		if sn == nil {
			continue
		}
		rel := sn.(enqueueRelease)
		if rel.Status == "" {
			rel.Status = v1alpha1.PhasePending
			status := map[string]v1alpha1.ModuleReleaseStatus{
				"status": {
					Phase:          v1alpha1.PhasePending,
					TransitionTime: ts,
					Message:        "",
				},
			}
			input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", rel.Name, object_patch.WithSubresource("/status"))
		}

		moduleReleases[rel.ModuleName] = append(moduleReleases[rel.ModuleName], rel)
	}

	for module, releases := range moduleReleases {
		sort.Sort(byVersion[enqueueRelease](releases))
		delete(fsModulesLinks, module)

		pred := newReleasePredictor(releases)

		pred.calculateRelease()

		// search symlink for module by regexp
		// module weight for a new version of the module may be different from the old one,
		// we need to find a symlink that contains the module name without looking at the weight prefix.
		currentModuleSymlink, err := findExistingModuleSymlink(symlinksDir, module)
		if err != nil {
			currentModuleSymlink = "900-" + module // fallback
		}

		if pred.currentReleaseIndex == len(pred.releases)-1 {
			// latest release deployed
			deployedRelease := pred.releases[pred.currentReleaseIndex]
			deckhouse_config.Service().AddModuleNameToSource(deployedRelease.ModuleName, deployedRelease.ModuleSource)

			// check symlink exists on FS, relative symlink
			modulePath := generateModulePath(module, deployedRelease.Version.String())
			if !isModuleExistsOnFS(symlinksDir, currentModuleSymlink, modulePath) {
				newModuleSymlink := path.Join(symlinksDir, fmt.Sprintf("%d-%s", deployedRelease.Weight, module))
				input.LogEntry.Debugf("Module %q is not exists on the filesystem. Restoring", module)
				err := enableModule(externalModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
				if err != nil {
					input.LogEntry.Errorf("Module restore failed: %v", err)
					suspendModuleVersionForRelease(input, deployedRelease, err, ts)
					continue
				}
				modulesChangedReason = "one of modules is not enabled"
			}
			continue
		}

		if len(pred.skippedPatchesIndexes) > 0 {
			for _, index := range pred.skippedPatchesIndexes {
				release := pred.releases[index]
				status := map[string]v1alpha1.ModuleReleaseStatus{
					"status": {
						Phase:          v1alpha1.PhaseSuperseded,
						TransitionTime: pred.ts,
						Message:        "",
					},
				}
				input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
			}
		}

		if pred.currentReleaseIndex >= 0 {
			release := pred.releases[pred.currentReleaseIndex]
			status := map[string]v1alpha1.ModuleReleaseStatus{
				"status": {
					Phase:          v1alpha1.PhaseSuperseded,
					TransitionTime: pred.ts,
					Message:        "",
				},
			}
			input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
		}

		if pred.desiredReleaseIndex >= 0 {
			release := pred.releases[pred.desiredReleaseIndex]

			modulePath := generateModulePath(module, release.Version.String())
			newModuleSymlink := path.Join(symlinksDir, fmt.Sprintf("%d-%s", release.Weight, module))

			err := enableModule(externalModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
			if err != nil {
				input.LogEntry.Errorf("Module deploy failed: %v", err)
				suspendModuleVersionForRelease(input, release, err, ts)
				continue
			}
			modulesChangedReason = "a new module release found"

			status := map[string]v1alpha1.ModuleReleaseStatus{
				"status": {
					Phase:          v1alpha1.PhaseDeployed,
					TransitionTime: pred.ts,
					Message:        "",
				},
			}
			input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
		}
	}
	if !fsSynchronized {
		if len(fsModulesLinks) > 0 {
			for module, moduleLinkPath := range fsModulesLinks {
				input.LogEntry.Warnf("Module %q has no releases. Purging from FS", module)
				_ = os.RemoveAll(moduleLinkPath)
			}
			modulesChangedReason = "the modules filesystem is not synchronized"
		}
		fsSynchronized = true
	}

	if modulesChangedReason != "" {
		input.LogEntry.Infof("Restarting Deckhouse because %s", modulesChangedReason)

		err := syscall.Kill(1, syscall.SIGUSR2)
		if err != nil {
			input.LogEntry.Errorf("Send SIGUSR2 signal failed: %s", err)
			return nil
		}
	}

	return nil
}

func suspendModuleVersionForRelease(input *go_hook.HookInput, release enqueueRelease, err error, ts metav1.Time) {
	if os.IsNotExist(err) {
		err = errors.New("not found")
	}
	status := map[string]v1alpha1.ModuleReleaseStatus{
		"status": {
			Phase:          v1alpha1.PhaseSuspended,
			TransitionTime: ts,
			Message:        fmt.Sprintf("Desired version of the module met problems: %s", err),
		},
	}
	input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
}

func findExistingModuleSymlink(rootPath, moduleName string) (string, error) {
	var symlinkPath string

	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)
	walkDir := func(path string, d os.DirEntry, err error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}

		symlinkPath = path
		return filepath.SkipDir
	}

	err := filepath.WalkDir(rootPath, walkDir)

	return symlinkPath, err
}

func isModuleExistsOnFS(symlinksDir, symlinkPath, modulePath string) bool {
	targetPath, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return false
	}

	if filepath.IsAbs(targetPath) {
		targetPath, err = filepath.Rel(symlinksDir, targetPath)
		if err != nil {
			return false
		}
	}

	return targetPath == modulePath
}

func enableModule(externalModulesDir, oldSymlinkPath, newSymlinkPath, modulePath string) error {
	if oldSymlinkPath != "" {
		if _, err := os.Lstat(oldSymlinkPath); err == nil {
			err = os.Remove(oldSymlinkPath)
			if err != nil {
				return err
			}
		}
	}

	if _, err := os.Lstat(newSymlinkPath); err == nil {
		err = os.Remove(newSymlinkPath)
		if err != nil {
			return err
		}
	}

	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(modulePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(modulePath, newSymlinkPath)
}

func generateModulePath(moduleName, version string) string {
	return path.Join("../", moduleName, "v"+version)
}

func filterRelease(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var release v1alpha1.ModuleRelease

	err := sdk.FromUnstructured(obj, &release)
	if err != nil {
		return nil, err
	}

	var releaseApproved bool
	if v, ok := release.Annotations["release.deckhouse.io/approved"]; ok {
		if v == "true" {
			releaseApproved = true
		}
	}
	if release.Spec.Weight == 0 {
		release.Spec.Weight = defaultModuleWeight
	}

	return enqueueRelease{
		Name:         release.Name,
		Version:      release.Spec.Version,
		Weight:       release.Spec.Weight,
		ModuleName:   release.Spec.ModuleName,
		ModuleSource: release.Labels["source"],
		Status:       release.Status.Phase,
		Approved:     releaseApproved,
	}, nil
}

func readModulesFromFS(dir string) (map[string]string, error) {
	moduleLinks, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	modules := make(map[string]string, len(moduleLinks))

	for _, moduleLink := range moduleLinks {
		index := strings.Index(moduleLink.Name(), "-")
		if index == -1 {
			continue
		}

		moduleName := moduleLink.Name()[index+1:]
		modules[moduleName] = path.Join(dir, moduleLink.Name())
	}

	return modules, nil
}

type enqueueRelease struct {
	Name         string
	Version      *semver.Version
	Weight       int
	ModuleName   string
	ModuleSource string
	Status       string
	Approved     bool
}

func (er enqueueRelease) GetVersion() *semver.Version {
	return er.Version
}

type releasePredictor struct {
	ts metav1.Time

	releases              []enqueueRelease
	currentReleaseIndex   int
	desiredReleaseIndex   int
	skippedPatchesIndexes []int
}

func newReleasePredictor(releases []enqueueRelease) *releasePredictor {
	return &releasePredictor{
		ts:       metav1.NewTime(time.Now().UTC()),
		releases: releases,

		currentReleaseIndex:   -1,
		desiredReleaseIndex:   -1,
		skippedPatchesIndexes: make([]int, 0),
	}
}

func (rp *releasePredictor) calculateRelease() {
	for index, rl := range rp.releases {
		switch rl.Status {
		case v1alpha1.PhaseDeployed:
			rp.currentReleaseIndex = index

		case v1alpha1.PhasePending:
			if rp.desiredReleaseIndex >= 0 {
				previousPredictedRelease := rp.releases[rp.desiredReleaseIndex]
				if previousPredictedRelease.Version.Major() != rl.Version.Major() {
					continue
				}

				if previousPredictedRelease.Version.Minor() != rl.Version.Minor() {
					continue
				}
				// it's a patch for predicted release, continue
				rp.skippedPatchesIndexes = append(rp.skippedPatchesIndexes, rp.desiredReleaseIndex)
			}

			// release is predicted to be Deployed
			rp.desiredReleaseIndex = index
		}
	}
}
