/*
Copyright 2024 Flant JSC

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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	cniCiliumPath    = "/opt/cni/bin/cilium-cni"
	ciliumConstraint = ">= 1.17.4"
)

func main() {
	// Start and init

	// Check cilium-cni binary existence
	//   if cilium-cni exist
	//     then check its version
	//       if cilium-cni version grate or equal 1.17.4 (semver)
	//         then exit 0
	//       if cilium-cni version lower 1.17.4 (semver) or we not be able to get version
	//         then check is wg interfaces present on node
	//           if we can not be able to get interfaces or determine its
	//             then ???
	//           if not present
	//             then exit 0
	//           if present
	//             then check kernel version
	//               if kernel version met
	//                 then exit 0
	//               if kernel version not met
	//                 then
	//                   error "kernel version need update to 6.8 or greater"
	//                   exit 1
	//   if cilium-cni not exist
	//     then check is wg interfaces present on node
	//       if we can not be able to get interfaces or determine its
	//         then ???
	//       if not present
	//         then exit 0
	//       if present
	//         then check kernel version
	//           if kernel version met
	//             then exit 0
	//           if kernel version not met
	//             then
	//               error "kernel version need update to 6.8 or greater"
	//               exit 1

	_, err := os.Stat(cniCiliumPath)
	if err == nil {
		isCiliumAlreadyUpgraded, err := checkCiliumVersionByCNI(cniCiliumPath, ciliumConstraint)
		if err != nil {
			log.Error("failed to check cilium version: %w", err)
		}
		if isCiliumAlreadyUpgraded {
			log.Info("Cilium is already upgraded, nothing to do")
			return
		}
	} else {
		log.Info("cilium-cni binary '%s' does not exist: %w", cniCiliumPath, err)
	}

	isWGPresent, err := checkWireGuardInterfacesOnNode()
	if err != nil {
		log.Fatal("failed to check WireGuard interfaces: %w", err)
	}
	if !isWGPresent {
		log.Info("WireGuard interfaces are not present on the node")
		return
	}

	wgKernelConstraint := os.Getenv("WG_KERNEL_CONSTRAINT")
	if wgKernelConstraint == "" {
		log.Fatal("ENV variable WG_KERNEL_CONSTRAINT must be set")
	}
	isKernelVersionMet, err := checkKernelVersionWGCiliumRequirements(wgKernelConstraint)
	if err != nil {
		log.Fatal("failed to check kernel version: %w", err)
	}
	if !isKernelVersionMet {
		log.Fatal("the kernel does not met the requirements, and need to be updated to 6.8 or greater")
	}
	log.Info("the kernel meets the requirements, nothing to do")
	return
}

func checkCiliumVersionByCNI(cniCiliumPath, ciliumConstraint string) (bool, error) {
	startPhrase := "Cilium CNI plugin"
	endPhrase := "go version"
	var versionStr string

	cmd := exec.Command(cniCiliumPath, "VERSION")

	output, err := cmd.CombinedOutput()

	if err != nil {
		return false, fmt.Errorf("cant execute '%s': %w", cniCiliumPath, err)
	}

	startIndex := strings.Index(string(output), startPhrase)

	if startIndex == -1 {
		return false, fmt.Errorf("the version of the cilium could not be identified")
	}

	substr := string(output)[startIndex+len(startPhrase):]

	endIndex := strings.Index(substr, endPhrase)

	if endIndex != -1 {
		versionStr = strings.TrimSpace(substr[:endIndex])
	} else {
		versionStr = strings.TrimSpace(substr)
	}

	ciliumVersionSM, err := semver.NewVersion(versionStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse cilium version '%s': %w", versionStr, err)
	}

	ciliumConstraintSM, err := semver.NewConstraint(ciliumConstraint)
	if err != nil {
		return false, fmt.Errorf("failed to parse cilium constraint '%s': %w", ciliumConstraint, err)
	}

	if !ciliumConstraintSM.Check(ciliumVersionSM) {
		return false, fmt.Errorf("the cilium is already at upgraded (%s %s)", ciliumVersionSM, ciliumConstraintSM)
	}

	return true, nil
}

func checkKernelVersionWGCiliumRequirements(kernelConstraint string) (bool, error) {
	utsname := unix.Utsname{}
	err := unix.Uname(&utsname)
	if err != nil {
		return false, fmt.Errorf("failed to get kernel version: %w", err)
	}
	kernelVersion := string(utsname.Release[:])

	kernelVersionSM, err := semver.NewVersion(strings.Split(kernelVersion, "-")[0])
	if err != nil {
		return false, fmt.Errorf("failed to parse kernel version '%s': %w", kernelVersion, err)
	}

	kernelConstraintSM, err := semver.NewConstraint(kernelConstraint)
	if err != nil {
		return false, fmt.Errorf("failed to parse kernel constraint '%s': %w", kernelConstraint, err)
	}

	if !kernelConstraintSM.Check(kernelVersionSM) {
		return false, fmt.Errorf("the kernel %s does not meet the requirements: %s", kernelVersion, kernelConstraint)
	}

	return true, nil
}

func checkWireGuardInterfacesOnNode() (bool, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return false, fmt.Errorf("failed to get link list: %v", err)
	}
	hasWgInterface := false
	for _, link := range links {
		if link.Type() == "wireguard" {
			hasWgInterface = true
			break
		}
	}
	if hasWgInterface {
		return true, nil
	}
	return false, nil
}
