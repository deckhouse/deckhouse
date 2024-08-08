/*
Copyright 2023 The Kubernetes Authors.
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

package iptables

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kubernetes-sigs/iptables-wrappers/internal/commands"
	"github.com/kubernetes-sigs/iptables-wrappers/internal/files"
)

// AlternativeSelector allows to configure a system to use iptables in
// nft or legacy mode.
type AlternativeSelector interface {
	// UseMode configures the system to use the selected iptables mode.
	UseMode(ctx context.Context, mode Mode) error
}

// BuildAlternativeSelector builds the proper iptablesAlternativeSelector depending
// on the machine's setup. It will use either `alternatives` or `update-alternatives` if present
// in the sbin folder. If none is present, it will manage iptables binaries by manually
// creating symlinks.
func BuildAlternativeSelector(sbinPath string) AlternativeSelector {
	if files.ExecutableExists(filepath.Join(sbinPath, "alternatives")) {
		return alternativesSelector{sbinPath: sbinPath}
	} else if files.ExecutableExists(filepath.Join(sbinPath, "update-alternatives")) {
		return updateAlternativesSelector{sbinPath: sbinPath}
	} else {
		// if we don't find any tool to managed the alternatives, handle it manually with symlinks
		return symlinkSelector{sbinPath: sbinPath}
	}
}

// updateAlternativesSelector manages an iptables setup by using the `update-alternatives` binary.
// This is most common for debian based OSs.
type updateAlternativesSelector struct {
	sbinPath string
}

func (u updateAlternativesSelector) UseMode(ctx context.Context, mode Mode) error {
	modeStr := string(mode)

	if err := commands.RunAndReadError(exec.CommandContext(ctx, "update-alternatives", "--set", "iptables", filepath.Join(u.sbinPath, "iptables-"+modeStr))); err != nil {
		return fmt.Errorf("update-alternatives iptables to mode %s: %v", modeStr, err)
	}

	if err := commands.RunAndReadError(exec.CommandContext(ctx, "update-alternatives", "--set", "ip6tables", filepath.Join(u.sbinPath, "ip6tables-"+modeStr))); err != nil {
		return fmt.Errorf("update-alternatives ip6tables to mode %s: %v", modeStr, err)
	}

	return nil
}

// alternativesSelector manages an iptables setup by using the `alternatives` binary.
// This is most common for fedora based OSs.
type alternativesSelector struct {
	sbinPath string
}

func (a alternativesSelector) UseMode(ctx context.Context, mode Mode) error {
	if err := commands.RunAndReadError(exec.CommandContext(ctx, "alternatives", "--set", "iptables", filepath.Join(a.sbinPath, "iptables-"+string(mode)))); err != nil {
		return fmt.Errorf("alternatives to update iptables to mode %s: %v", string(mode), err)
	}
	return nil
}

// symlinkSelector  manages an iptables setup by manually creating symlinks
// that point to the proper "mode" binaries.
// It configures: `iptables`, `iptables-save`, `iptables-restore`,
// `ip6tables`, `ip6tables-save` and `ip6tables-restore`.
type symlinkSelector struct {
	sbinPath string
}

func (s symlinkSelector) UseMode(ctx context.Context, mode Mode) error {
	modeStr := string(mode)
	xtablesForModePath := XtablesPath(s.sbinPath, mode)
	cmds := []string{"iptables", "iptables-save", "iptables-restore", "ip6tables", "ip6tables-save", "ip6tables-restore"}

	for _, cmd := range cmds {
		cmdPath := filepath.Join(s.sbinPath, cmd)
		// If deleting fails, ignore it and try to create symlink regardless
		_ = os.RemoveAll(cmdPath)

		if err := os.Symlink(xtablesForModePath, cmdPath); err != nil {
			return fmt.Errorf("creating %s symlink for mode %s: %v", cmd, modeStr, err)
		}
	}

	return nil
}
