// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

func (pc *Checker) CheckStaticNodeSystemRequirements(ctx context.Context) error {
	if app.PreflightSkipSystemRequirementsCheck {
		log.DebugLn("System requirements check is skipped")
		return nil
	}

	ramKb, err := extractRAMCapacityFromNode(ctx, pc.nodeInterface)
	if err != nil {
		return err
	}

	coresCount, err := extractCPULogicalCoresCountFromNode(ctx, pc.nodeInterface)
	if err != nil {
		return err
	}

	minimumRequiredFoldersSizesGB := map[string]int{}
	if !pc.installConfig.Registry.IsDirect() {
		minimumRequiredFoldersSizesGB["/opt/deckhouse/system-registry"] = minimumRequiredRegistryDiskSizeGB
	}

	checkDiskSizeFailures, err := checkDiskSize(
		pc.nodeInterface,
		minimumRequiredFoldersSizesGB,
	)

	if err != nil {
		return err
	}

	failures := make([]string, 0)
	if len(checkDiskSizeFailures) > 0 {
		failures = append(failures, checkDiskSizeFailures...)
	}

	if coresCount < minimumRequiredCPUCores {
		failures = append(failures, fmt.Sprintf(
			" - System requirements mandate at least %d CPU(s) on the node, but it has %d",
			minimumRequiredCPUCores,
			coresCount,
		))
	}

	if ramKb < minimumRequiredMemoryMB*1024 {
		failures = append(failures, fmt.Sprintf(
			" - System requirements mandate at least %d MiB of RAM on the node, but it has %d MiB",
			minimumRequiredMemoryMB,
			ramKb/1024,
		))
	}

	if len(failures) > 0 {
		return fmt.Errorf("Deckhouse system requirements are not met by your current configuration:\n%s", strings.Join(failures, ";\n"))
	}

	return nil
}

func extractRAMCapacityFromNode(ctx context.Context, sshCl node.Interface) (int, error) {
	cmd := sshCl.Command("cat", "/proc/meminfo")
	memInfo, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("Failed to read MemTotal from /proc/meminfo: %w", err)
	}

	submatch := regexp.MustCompile(`^MemTotal:\s*(\d+)\s.B`).FindSubmatch(memInfo)
	ramKb, err := strconv.Atoi(string(submatch[1]))
	if err != nil {
		return 0, fmt.Errorf("Failed to parse MemTotal from /proc/meminfo: %w", err)
	}
	return ramKb, nil
}

func extractCPULogicalCoresCountFromNode(ctx context.Context, nodeInterface node.Interface) (int, error) {
	cmd := nodeInterface.Command("cat", "/proc/cpuinfo")
	stdout, _, err := cmd.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("Failed to read CPU info from /proc/cpuinfo: %w", err)
	}

	count, err := logicalCoresCountFromCPUInfo(stdout)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse CPU info from /proc/cpuinfo: %w", err)
	}
	return count, nil
}

func logicalCoresCountFromCPUInfo(cpuinfo []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(cpuinfo))
	processors := make(map[string]struct{})
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}

		field := strings.SplitN(line, ": ", 2)
		if strings.TrimSpace(field[0]) == "processor" {
			processors[strings.TrimSpace(field[1])] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("Failed to parse cpu info from /proc/cpuinfo: %w", err)
	}

	return len(processors), nil
}

// extractAvailableDisksSizeFromNode retrieves the Available disk size information from the node.
func extractAvailableDisksSizeFromNode(nodeInterface node.Interface) ([]byte, error) {
	// Execute the command to get Available disk size info in MB and path.
	// user@host:~# df -h -BM | awk 'NR > 1 {print $4, "\t", $6}'
	// 793M     /run
	// 93540M   /
	// 3971M    /dev/shm
	// 5M       /run/lock
	// 795M     /run/user/1000
	cmd := nodeInterface.Command("df -h -BM | awk 'NR > 1 {print $4, \"\\t\", $6}'")
	stdout, _, err := cmd.Output(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get disk size info: %w", err)
	}
	return stdout, nil
}

// parseDiskSizeInfo processes the disk size info and returns a map of paths to their sizes in MB.
func parseDiskSizeInfo(diskInfo []byte) (map[string]int64, error) {
	scanner := bufio.NewScanner(bytes.NewReader(diskInfo))
	sizeInMB := make(map[string]int64)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("failed to parse disk size info: expected 2 fields, got %d", len(fields))
		}

		// Remove the 'M' from the size and convert it to int64.
		sizeMBStr := strings.ReplaceAll(fields[0], "M", "")
		path := fields[1]

		var sizeMB int64
		if _, err := fmt.Sscanf(sizeMBStr, "%d", &sizeMB); err != nil {
			return nil, fmt.Errorf("failed to parse disk size value '%s': %v", sizeMBStr, err)
		}

		sizeInMB[path] = sizeMB
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read disk size info: %v", err)
	}

	return sizeInMB, nil
}

// getRelationFoldersByDisk returns a mapping of disks to their respective folders.
func getRelationFoldersByDisk(disks []string, folders []string) (map[string][]string, error) {
	// Sort disks by depth (number of slashes) and length.
	sort.Slice(disks, func(i, j int) bool {
		cleanedI := filepath.Clean(disks[i])
		cleanedJ := filepath.Clean(disks[j])

		countI := strings.Count(cleanedI, "/")
		countJ := strings.Count(cleanedJ, "/")
		if countI == countJ {
			return len(cleanedI) > len(cleanedJ)
		}
		return countI > countJ
	})

	relations := make(map[string][]string)
	for _, folder := range folders {
		found := false
		for _, disk := range disks {
			if strings.HasPrefix(folder, disk) {
				relations[disk] = append(relations[disk], folder)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("failed to determine disk relation: unknown disk path for folder '%s'", folder)
		}
	}
	return relations, nil
}

// checkDiskSize checks if the available disk sizes meet the minimum requirements for the folders.
func checkDiskSize(nodeInterface node.Interface, minimumRequiredFoldersSizesGB map[string]int) ([]string, error) {
	// Extract disk size information.
	diskInfo, err := extractAvailableDisksSizeFromNode(nodeInterface)
	if err != nil {
		return nil, err
	}

	// Parse the disk size info into a map.
	diskSizeInfo, err := parseDiskSizeInfo(diskInfo)
	if err != nil {
		return nil, err
	}

	// Collect disk and folder paths.
	var disks []string
	for disk := range diskSizeInfo {
		disks = append(disks, disk)
	}

	var folders []string
	for folder := range minimumRequiredFoldersSizesGB {
		folders = append(folders, folder)
	}

	// Get folder-disk relationships.
	relations, err := getRelationFoldersByDisk(disks, folders)
	if err != nil {
		return nil, err
	}

	failures := []string{}
	for disk, folders := range relations {
		diskAvailableSizeGB := diskSizeInfo[disk] / 1024 // Convert MB to GB
		sumExpectedSizeGB := 0
		folderInfo := make([]string, 0, len(folders))

		// Calculate the expected size for the folders on this disk.
		for _, folder := range folders {
			folderExpectedSizeGB := minimumRequiredFoldersSizesGB[folder]
			sumExpectedSizeGB += folderExpectedSizeGB
			folderInfo = append(folderInfo, fmt.Sprintf("%d GB for the folder '%s'", folderExpectedSizeGB, folder))
		}

		// Check if the expected size exceeds the available size.
		if int64(sumExpectedSizeGB) > diskAvailableSizeGB {
			failures = append(
				failures,
				fmt.Sprintf(
					" - System requirements mandate at least %s, but only %d GB of available space is present on the disk '%s'",
					strings.Join(folderInfo, ", "),
					diskAvailableSizeGB,
					disk,
				),
			)
		}
	}
	return failures, nil
}
