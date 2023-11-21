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

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	deckhouse_config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/modules/005-external-module-manager/hooks/internal/apis/v1alpha1"
)

const (
	policyNotFound   = "Release isn't associated with any update policy"
	manualApproval   = "Waiting for manual approval"
	waitingForWindow = "Release is waiting for the update window: %s"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// check symlinks exist on the startup
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 5,
	},
	Queue: "/modules/external-module-source/apply-release",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "policies",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleUpdatePolicy",
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterPolicy,
		},
		{
			Name:                         "releases",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleRelease",
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterRelease,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_module_releases",
			Crontab: "*/15 * * * * *",
		},
	},
}, applyModuleRelease)

var (
	fsSynchronized = false
)

func applyModuleRelease(input *go_hook.HookInput) error {
	var modulesChangedReason string
	var fsModulesLinks map[string]string

	snapReleases := input.Snapshots["releases"]
	snapPolicies := input.Snapshots["policies"]

	externalModulesDir := os.Getenv("EXTERNAL_MODULES_DIR")
	if externalModulesDir == "" {
		input.LogEntry.Warn("EXTERNAL_MODULES_DIR is not set")
		return nil
	}
	// directory for symlinks will actual versions to all external-modules
	symlinksDir := filepath.Join(externalModulesDir, "modules")
	ts := time.Now().UTC()

	// run only once on startup
	if !fsSynchronized {
		var err error
		fsModulesLinks, err = readModulesFromFS(symlinksDir)
		if err != nil {
			input.LogEntry.Errorf("Could not read modules from fs: %s", err)
			return nil
		}
	}

	policies := make(map[string]*modulePolicy)

	for _, pol := range snapPolicies {
		policy := pol.(v1alpha1.ModuleUpdatePolicy)
		policies[policy.Name] = &modulePolicy{
			spec: policy.Spec,
		}
	}

	moduleReleases := make(map[string][]enqueueRelease, 0)
	for _, sn := range snapReleases {
		if sn == nil {
			continue
		}
		rel := sn.(enqueueRelease)

		_, foundPolicy := policies[rel.ModuleUpdatePolicy]
		switch rel.Status {
		case "":
			if !foundPolicy {
				setReleasePhaseWithMsg(input, rel, v1alpha1.PhasePolicyUndefined, policyNotFound, ts)
				rel.Status = v1alpha1.PhasePolicyUndefined
			} else {
				setReleasePhaseWithMsg(input, rel, v1alpha1.PhasePending, "", ts)
				rel.Status = v1alpha1.PhasePending
			}
		case v1alpha1.PhasePending:
			if !foundPolicy {
				setReleasePhaseWithMsg(input, rel, v1alpha1.PhasePolicyUndefined, policyNotFound, ts)
				rel.Status = v1alpha1.PhasePolicyUndefined
			}
		case v1alpha1.PhasePolicyUndefined:
			if foundPolicy {
				setReleasePhaseWithMsg(input, rel, v1alpha1.PhasePending, "", ts)
				rel.Status = v1alpha1.PhasePending
			}
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
					setSuspendedModuleVersionForRelease(input, deployedRelease, err, ts)
					continue
				}
				modulesChangedReason = "one of modules is not enabled"
			}
			continue
		}

		if len(pred.skippedPatchesIndexes) > 0 {
			for _, index := range pred.skippedPatchesIndexes {
				release := pred.releases[index]
				setReleasePhaseWithMsg(input, release, v1alpha1.PhaseSuperseded, "", pred.ts)
			}
		}

		if pred.desiredReleaseIndex >= 0 {
			release := pred.releases[pred.desiredReleaseIndex]

			if releasePolicy, found := policies[release.ModuleUpdatePolicy]; found {
				// manual and not approved
				if releasePolicy.spec.Update.Mode == "Manual" && !release.Approved {
					setReleasePhaseWithMsg(input, release, v1alpha1.PhasePending, manualApproval, ts)
					continue
				}

				// auto and not in time
				if releasePolicy.spec.Update.Mode == "Auto" && !releasePolicy.spec.Update.Windows.IsAllowed(ts) {
					setReleasePhaseWithMsg(input, release, v1alpha1.PhasePending, fmt.Sprintf(waitingForWindow, releasePolicy.spec.Update.Windows.NextAllowedTime(ts)), ts)
					continue
				}

				modulePath := generateModulePath(module, release.Version.String())
				newModuleSymlink := path.Join(symlinksDir, fmt.Sprintf("%d-%s", release.Weight, module))

				err := enableModule(externalModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
				if err != nil {
					input.LogEntry.Errorf("Module deploy failed: %v", err)
					setSuspendedModuleVersionForRelease(input, release, err, ts)
					continue
				}

				// after deploying a new release, mark previous one (if any) as superseded
				if pred.currentReleaseIndex >= 0 {
					setReleasePhaseWithMsg(input, pred.releases[pred.currentReleaseIndex], v1alpha1.PhaseSuperseded, "", pred.ts)
				}

				modulesChangedReason = "a new module release found"
				setReleasePhaseWithMsg(input, release, v1alpha1.PhaseDeployed, "", pred.ts)
			} else {
				setReleasePhaseWithMsg(input, release, v1alpha1.PhasePolicyUndefined, policyNotFound, ts)
			}
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

func setReleasePhaseWithMsg(input *go_hook.HookInput, release enqueueRelease, phase, message string, ts time.Time) {
	if release.Status != phase {
		status := map[string]v1alpha1.ModuleReleaseStatus{
			"status": {
				Phase:          phase,
				TransitionTime: ts,
				Message:        message,
			},
		}
		input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
	} else if release.StatusMessage != message {
		status := map[string]v1alpha1.ModuleReleaseStatus{
			"status": {
				Phase:          phase,
				TransitionTime: release.StatusTransitionTime,
				Message:        message,
			},
		}
		input.PatchCollector.MergePatch(status, "deckhouse.io/v1alpha1", "ModuleRelease", "", release.Name, object_patch.WithSubresource("/status"))
	}
}

func setSuspendedModuleVersionForRelease(input *go_hook.HookInput, release enqueueRelease, err error, ts time.Time) {
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
		Name:                 release.Name,
		Version:              release.Spec.Version,
		Weight:               release.Spec.Weight,
		ModuleName:           release.Spec.ModuleName,
		ModuleSource:         release.Labels["source"],
		ModuleUpdatePolicy:   release.Labels["module-update-policy"],
		Status:               release.Status.Phase,
		StatusMessage:        release.Status.Message,
		StatusTransitionTime: release.Status.TransitionTime,
		Approved:             releaseApproved,
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
	Name                 string
	Version              *semver.Version
	Weight               int
	ModuleName           string
	ModuleSource         string
	ModuleUpdatePolicy   string
	Status               string
	StatusMessage        string
	StatusTransitionTime time.Time
	Approved             bool
}

func (er enqueueRelease) GetVersion() *semver.Version {
	return er.Version
}

type releasePredictor struct {
	ts time.Time

	releases              []enqueueRelease
	currentReleaseIndex   int
	desiredReleaseIndex   int
	skippedPatchesIndexes []int
}

func newReleasePredictor(releases []enqueueRelease) *releasePredictor {
	return &releasePredictor{
		ts:       time.Now().UTC(),
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
