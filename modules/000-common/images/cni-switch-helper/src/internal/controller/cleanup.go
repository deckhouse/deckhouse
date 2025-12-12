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

package controller

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RunCleanup executes the node-level cleanup for a specific CNI.
func RunCleanup(ctx context.Context, currentCNI string) error {
	logger := log.FromContext(ctx)
	logger.Info("Running cleanup for CNI", "cni", currentCNI)

	switch currentCNI {
	case "flannel":
		return cleanupFlannel(ctx)
	case "cilium":
		return cleanupCilium(ctx)
	case "simple-bridge":
		return cleanupSimpleBridge(ctx)
	default:
		return fmt.Errorf("unsupported CNI for cleanup: %s", currentCNI)
	}
}

func cleanupFlannel(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("flannel-cleanup")

	deleteInterfaces(logger, []string{"cni0", "flannel.1"})

	if err := deleteConfigFiles(logger, "flannel"); err != nil {
		logger.Error(err, "Failed to delete config files")
	}

	// Clean up directories
	removeDirectories(logger, []string{
		"/var/lib/cni/flannel",
		"/var/lib/cni/networks",
		"/var/lib/cni/results",
	})

	// Remove flannel subnet file
	subnetFile := "/run/flannel/subnet.env"
	if err := os.Remove(subnetFile); err != nil && !os.IsNotExist(err) {
		logger.Error(err, "Failed to delete subnet file", "file", subnetFile)
	}

	patterns := []string{
		"FLANNEL-",
		"CNI-",
		"KUBE-",
	}
	if err := cleanIptablesByPatterns(logger, patterns); err != nil {
		logger.Error(err, "Failed to clean iptables rules for flannel")
	}

	logger.Info("Flannel cleanup finished")
	return nil
}

func cleanupCilium(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("cilium-cleanup")

	// Use cilium-dbg utility for deep cleanup (eBPF maps, progs, etc.)
	if err := runCommand(logger, "/sbin/cilium-dbg", "post-uninstall-cleanup", "--force"); err != nil {
		logger.Error(err, "cilium-dbg post-uninstall-cleanup failed, continuing with manual cleanup")
	} else {
		logger.Info("cilium-dbg post-uninstall-cleanup finished successfully")
	}

	if err := deleteConfigFiles(logger, "cilium"); err != nil {
		logger.Error(err, "Failed to delete config files")
	}

	// Clean up directories
	removeDirectories(logger, []string{
		"/var/lib/cni/networks",
		"/var/lib/cni/results",
	})

	// Cilium creates a lot of chains, usually prefixed with CILIUM_
	patterns := []string{"CILIUM"}
	if err := cleanIptablesByPatterns(logger, patterns); err != nil {
		logger.Error(err, "Failed to clean iptables rules for Cilium")
	}

	logger.Info("Cilium cleanup finished")
	return nil
}

func cleanupSimpleBridge(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("simple-bridge-cleanup")

	deleteInterfaces(logger, []string{"cni0"})

	if err := deleteConfigFiles(logger, "simple-bridge"); err != nil {
		logger.Error(err, "Failed to delete config files")
	}

	// Clean up directories
	removeDirectories(logger, []string{
		"/var/lib/cni/networks",
		"/var/lib/cni/results",
	})

	patterns := []string{
		"CNI-",
		"KUBE-",
	}
	if err := cleanIptablesByPatterns(logger, patterns); err != nil {
		logger.Error(err, "Failed to clean iptables rules for simple-bridge")
	}

	logger.Info("simple-bridge cleanup finished")
	return nil
}

// cleanIptablesByPatterns removes rules and chains containing ANY of the patterns.
func cleanIptablesByPatterns(logger logr.Logger, patterns []string) error {
	logger.Info("Cleaning iptables rules", "patterns", patterns)

	// Save current rules
	var rulesBytes bytes.Buffer
	cmdSave := exec.Command("/sbin/iptables-save")
	cmdSave.Stdout = &rulesBytes
	if err := cmdSave.Run(); err != nil {
		return fmt.Errorf("iptables-save failed: %w", err)
	}

	rules := rulesBytes.String()
	var newRulesBuilder strings.Builder
	removedCount := 0

	lines := strings.Split(rules, "\n")
	for _, line := range lines {
		// Always keep table declarations, COMMITs, and comments.
		// These lines start with '*', '#', or are exactly "COMMIT".
		if strings.HasPrefix(line, "*") || line == "COMMIT" || strings.HasPrefix(line, "#") {
			newRulesBuilder.WriteString(line)
			newRulesBuilder.WriteString("\n")
			continue
		}

		// For all other lines (chain declarations and actual rules),
		// remove if they contain any of the patterns.
		shouldRemove := false
		for _, p := range patterns {
			if strings.Contains(line, p) {
				shouldRemove = true
				break
			}
		}

		if shouldRemove {
			removedCount++
			continue
		}
		newRulesBuilder.WriteString(line)
		newRulesBuilder.WriteString("\n")
	}

	if removedCount == 0 {
		logger.Info("No iptables rules found matching patterns")
		return nil
	}

	logger.Info("Found rules/chains to remove", "count", removedCount)

	// Restore filtered rules
	cmdRestore := exec.Command("/sbin/iptables-restore")
	cmdRestore.Stdin = strings.NewReader(newRulesBuilder.String())
	var stderr bytes.Buffer
	cmdRestore.Stderr = &stderr

	if err := cmdRestore.Run(); err != nil {
		return fmt.Errorf("iptables-restore failed: %w; stderr: %s", err, stderr.String())
	}

	logger.Info("Successfully cleaned iptables rules")
	return nil
}

func deleteInterfaces(logger logr.Logger, interfaces []string) {
	for _, iface := range interfaces {
		if err := runCommand(logger, "/sbin/ip", "link", "delete", iface); err != nil {
			// It is fine if interface does not exist
			logger.Info("Could not delete interface (likely did not exist)", "interface", iface)
		} else {
			logger.Info("Successfully deleted interface", "interface", iface)
		}
	}
}

func deleteConfigFiles(logger logr.Logger, nameContains string) error {
	configDir := "/etc/cni/net.d/"
	files, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir %s: %w", configDir, err)
	}

	for _, f := range files {
		if strings.Contains(f.Name(), nameContains) {
			fullPath := filepath.Join(configDir, f.Name())
			if err := os.Remove(fullPath); err != nil {
				logger.Error(err, "Failed to remove CNI config file", "file", fullPath)
			} else {
				logger.Info("Removed CNI config file", "file", fullPath)
			}
		}
	}
	return nil
}

// runCommand executes a shell command and logs its output.
func runCommand(logger logr.Logger, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Info("Running command", "command", cmd.String())
	err := cmd.Run()

	if err != nil {
		logger.Error(err, "Command execution failed", "stdout", stdout.String(), "stderr", stderr.String())
		return fmt.Errorf("command %s failed: %w; stderr: %s", cmd.String(), err, stderr.String())
	}
	return nil
}

func removeDirectories(logger logr.Logger, dirs []string) {
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			logger.Error(err, "Failed to delete directory", "dir", dir)
		} else {
			logger.Info("Removed directory", "dir", dir)
		}
	}
}
