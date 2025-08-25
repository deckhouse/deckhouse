/*
Copyright 2025 Flant JSC

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

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	// /opt/cni/bin is mounted to /hostbin
	cniCiliumPath       = "/hostbin/cilium-cni"
	ciliumConstraintDef = ">= 1.17.4"
)

func main() {
	ciliumConstraint := os.Getenv("CILIUM_CONSTRAINT")
	if ciliumConstraint == "" {
		log.Info(fmt.Sprintf("ENV variable CILIUM_CONSTRAINT is not set, use default value '%s'", ciliumConstraintDef))
		ciliumConstraint = ciliumConstraintDef
	}
	if exist, err := isCiliumBinaryExists(cniCiliumPath); !exist {
		if err != nil {
			log.Warn(fmt.Sprintf("failed to check cilium-cni binary '%s': %v", cniCiliumPath, err))
		} else {
			log.Info(fmt.Sprintf("cilium-cni binary '%s' does not exist", cniCiliumPath))
		}
	} else if cniCiliumVersionStr, err := getCiliumVersionByCNI(cniCiliumPath); err != nil {
		log.Warn("failed to get cilium version", log.Err(err))
	} else if isCiliumAlreadyUpgraded, err := checkCiliumVersion(cniCiliumVersionStr, ciliumConstraint); err != nil {
		log.Warn("failed to check cilium version", log.Err(err))
	} else if isCiliumAlreadyUpgraded {
		log.Info("cilium is already upgraded, there is nothing to do")
		return
	}

	isWGPresent, err := checkWireGuardInterfacesOnNode()
	if err != nil {
		log.Warn("failed to check for WireGuard interfaces. If the WireGuard interfaces are present on the node and the kernel version is less than 6.8, there may be an issue with 'BPF is too large'", log.Err(err))
		return
	}
	if !isWGPresent {
		log.Info("WireGuard interfaces are not present on the node")
		return
	}
	log.Info("WireGuard interfaces are present on the node")

	wgKernelConstraint := os.Getenv("WG_KERNEL_CONSTRAINT")
	if wgKernelConstraint == "" {
		log.Warn("ENV variable WG_KERNEL_CONSTRAINT must be set")
		os.Exit(1)
	}

	kernelVersion, err := getCurrentKernelVersion()
	if err != nil {
		log.Warn("failed to get current kernel version. If the kernel version is less than 6.8, there may be an issue with 'BPF is too large'", log.Err(err))
		return
	}
	isKernelVersionMeet, err := checkKernelVersionWGCiliumRequirements(kernelVersion, wgKernelConstraint)
	if err != nil {
		log.Warn("failed to check kernel version. If the kernel version is less than 6.8, there may be an issue with 'BPF is too large'", log.Err(err))
		return
	}
	if !isKernelVersionMeet {
		log.Warn("the kernel does not meet the requirements and needs to be updated to version 6.8 or higher")
		os.Exit(1)
	}
	log.Info("the kernel meets the requirements, there is nothing to do")
	return
}

func isCiliumBinaryExists(cniCiliumPath string) (bool, error) {
	_, err := os.Stat(cniCiliumPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getCiliumVersionByCNI(cniCiliumPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cniCiliumPath, "VERSION")
	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("command '%s VERSION' timed out", cniCiliumPath)
	}
	if err != nil {
		return "", fmt.Errorf("cant execute '%s VERSION': %w", cniCiliumPath, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func checkCiliumVersion(cniCiliumVersionStr, ciliumConstraint string) (bool, error) {
	startPhrase := "Cilium CNI plugin"
	endPhrase := "go version"
	var versionStr string

	startPhraseIndex := strings.Index(cniCiliumVersionStr, startPhrase)

	if startPhraseIndex == -1 {
		return false, fmt.Errorf("the version of the cilium could not be identified")
	}

	substr := cniCiliumVersionStr[startPhraseIndex+len(startPhrase):]

	endPhraseIndex := strings.Index(substr, endPhrase)
	lineEndIndex := strings.IndexAny(substr, "\r\n")

	endIndex := len(substr)

	if endPhraseIndex != -1 && endPhraseIndex < endIndex {
		endIndex = endPhraseIndex
	}
	if lineEndIndex != -1 && lineEndIndex < endIndex {
		endIndex = lineEndIndex
	}

	versionStr = strings.TrimSpace(substr[:endIndex])

	ciliumVersionSM, err := semver.NewVersion(versionStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse cilium version '%s': %w", versionStr, err)
	}

	ciliumConstraintSM, err := semver.NewConstraint(ciliumConstraint)
	if err != nil {
		return false, fmt.Errorf("failed to parse cilium constraint '%s': %w", ciliumConstraint, err)
	}

	if !ciliumConstraintSM.Check(ciliumVersionSM) {
		return false, fmt.Errorf("the Cilium version has not been upgraded yet. The condition (%s %s) has not been met", ciliumVersionSM.String(), ciliumConstraintSM.String())
	}

	return true, nil
}

func checkWireGuardInterfacesOnNode() (bool, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return false, fmt.Errorf("failed to get link list: %v", err)
	}
	for _, link := range links {
		if link.Type() == "wireguard" {
			return true, nil
		}
	}
	return false, nil
}

func getCurrentKernelVersion() (string, error) {
	utsname := unix.Utsname{}
	err := unix.Uname(&utsname)
	if err != nil {
		return "", fmt.Errorf("failed to get kernel version: %w", err)
	}
	return strings.TrimSpace(string(bytes.TrimRight(utsname.Release[:], "\x00"))), nil
}

func checkKernelVersionWGCiliumRequirements(kernelVersion, kernelConstraint string) (bool, error) {
	kernelVersionSM, err := semver.NewVersion(strings.Split(kernelVersion, "-")[0])
	if err != nil {
		return false, fmt.Errorf("failed to parse kernel version '%s': %w", kernelVersion, err)
	}

	kernelConstraintSM, err := semver.NewConstraint(kernelConstraint)
	if err != nil {
		return false, fmt.Errorf("failed to parse kernel constraint '%s': %w", kernelConstraint, err)
	}

	if !kernelConstraintSM.Check(kernelVersionSM) {
		return false, nil
	}

	return true, nil
}
