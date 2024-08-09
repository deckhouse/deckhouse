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
	"errors"

	"github.com/kubernetes-sigs/iptables-wrappers/internal/files"
)

// DetectBinaryDir tries to detect the `iptables` location in
// either /usr/sbin or /sbin. If it's not there, it returns an error.
func DetectBinaryDir() (string, error) {
	if files.ExecutableExists("/usr/sbin/iptables") {
		return "/usr/sbin", nil
	} else if files.ExecutableExists("/sbin/iptables") {
		return "/sbin", nil
	} else {
		return "", errors.New("iptables is not present in either /usr/sbin or /sbin")
	}
}

// Mode represents the two different modes iptables can be
// configured to: nft or legacy. In string form it can be used to
// to complete all `iptables-*` commands.
type Mode string

const (
	legacy Mode = "legacy"
	nft    Mode = "nft"
)

// DetectMode inspects the current iptables entries and tries to
// guess which iptables mode is being used: legacy or nft
func DetectMode(ctx context.Context, iptables Installation) Mode {
	// This method ignores all errors, this is on purpose. We execute all commands
	// and try to detect patterns in a best effort basis. If somthing fails,
	// continue with the next step. Worse case scenario if everything fails,
	// default to nft.

	// In kubernetes 1.17 and later, kubelet will have created at least
	// one chain in the "mangle" table (either "KUBE-IPTABLES-HINT" or
	// "KUBE-KUBELET-CANARY"), so check that first, against
	// iptables-nft, because we can check that more efficiently and
	// it's more common these days.
	rulesOutput := &bytes.Buffer{}
	_ = iptables.NFTSave(ctx, rulesOutput, "-t", "mangle")
	if hasKubeletChains(rulesOutput.Bytes()) {
		return nft
	}
	rulesOutput.Reset()
	_ = iptables.NFTSaveIP6(ctx, rulesOutput, "-t", "mangle")
	if hasKubeletChains(rulesOutput.Bytes()) {
		return nft
	}
	rulesOutput.Reset()

	// Check for kubernetes 1.17-or-later with iptables-legacy. We
	// can't pass "-t mangle" to iptables-legacy-save because it would
	// cause the kernel to create that table if it didn't already
	// exist, which we don't want. So we have to grab all the rules.
	_ = iptables.LegacySave(ctx, rulesOutput)
	if hasKubeletChains(rulesOutput.Bytes()) {
		return legacy
	}
	rulesOutput.Reset()
	_ = iptables.LegacySaveIP6(ctx, rulesOutput)
	if hasKubeletChains(rulesOutput.Bytes()) {
		return legacy
	}

	// If we can't detect any of the 2 patterns, default to nft.
	return nft
}
