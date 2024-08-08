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
	"bytes"
	"context"
	"os/exec"
	"path/filepath"

	"github.com/kubernetes-sigs/iptables-wrappers/internal/commands"
)

const (
	xtablesNFTMultiBinaryName    = "xtables-nft-multi"
	xtablesLegacyMultiBinaryName = "xtables-legacy-multi"
)

// Installation represents the set of iptables-*-save binaries installed in a machine.
// It is expected the machine supports both nft and legacy modes. This can be implemented by
// calling directly iptables-*-save, xtables, etc. The implementation should accept the same
// command arguments as the mentioned binaries.
type Installation interface {
	// LegacySave runs a iptables-legacy-save command
	LegacySave(ctx context.Context, out *bytes.Buffer, args ...string) error
	// LegacySaveIP6 runs a ip6tables-legacy-save command
	LegacySaveIP6(ctx context.Context, out *bytes.Buffer, args ...string) error
	// NFTSave runs a iptables-nft-save command
	NFTSave(ctx context.Context, out *bytes.Buffer, args ...string) error
	// NFTSaveIP6 runs a ip6tables-nft-save command
	NFTSaveIP6(ctx context.Context, out *bytes.Buffer, args ...string) error
}

func NewXtablesMultiInstallation(sbinPath string) XtablesMulti {
	return XtablesMulti{
		nftBinary:    filepath.Join(sbinPath, xtablesNFTMultiBinaryName),
		legacyBinary: filepath.Join(sbinPath, xtablesLegacyMultiBinaryName),
	}
}

// XtablesMulti allows to run iptables commands using xtables-*-multi.
// It implements iptablesInstallation.
type XtablesMulti struct {
	nftBinary    string
	legacyBinary string
}

func (x XtablesMulti) LegacySave(ctx context.Context, out *bytes.Buffer, args ...string) error {
	return x.exec(ctx, out, x.legacyBinary, "iptables-save", args...)
}

func (x XtablesMulti) LegacySaveIP6(ctx context.Context, out *bytes.Buffer, args ...string) error {
	return x.exec(ctx, out, x.legacyBinary, "ip6tables-save", args...)
}

func (x XtablesMulti) NFTSave(ctx context.Context, out *bytes.Buffer, args ...string) error {
	return x.exec(ctx, out, x.nftBinary, "iptables-save", args...)
}

func (x XtablesMulti) NFTSaveIP6(ctx context.Context, out *bytes.Buffer, args ...string) error {
	return x.exec(ctx, out, x.nftBinary, "ip6tables-save", args...)
}

func (x XtablesMulti) exec(ctx context.Context, out *bytes.Buffer, multiBinary, command string, args ...string) error {
	allArgs := make([]string, 0, len(args)+1)
	allArgs = append(allArgs, command)
	allArgs = append(allArgs, args...)

	c := exec.CommandContext(ctx, multiBinary, allArgs...)
	c.Stdout = out

	return commands.RunAndReadError(c)
}

// XtablesPath returns the path to the `xtable-<mode>-multi binary
func XtablesPath(sbinPath string, mode Mode) string {
	return filepath.Join(sbinPath, "xtables-"+string(mode)+"-multi")
}
