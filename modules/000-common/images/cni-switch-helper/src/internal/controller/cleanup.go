package controller

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

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
	logger.Info("Starting Flannel cleanup")

	// Delete flannel interface
	if err := runCommand(logger, "ip", "link", "delete", "flannel.1"); err != nil {
		// It's okay if the interface doesn't exist
		logger.Info("Could not delete flannel.1 interface, it might not exist. This is probably okay.")
	} else {
		logger.Info("Successfully deleted flannel.1 interface")
	}

	// Delete CNI config file
	cniConfigFile := "/etc/cni/net.d/10-flannel.conflist"
	if err := runCommand(logger, "rm", "-f", cniConfigFile); err != nil {
		logger.Error(err, "Failed to delete Flannel CNI config file", "file", cniConfigFile)
		return err
	}
	logger.Info("Successfully deleted Flannel CNI config file", "file", cniConfigFile)

	// TODO: Implement more robust iptables cleanup.
	// This requires finding the specific rules added by flannel and deleting them,
	// rather than flushing entire chains.
	// e.g., iptables-save | grep flannel | iptables -t nat -D ...
	logger.Info("Skipping iptables cleanup for now (TODO)")

	logger.Info("Flannel cleanup finished")
	return nil
}

func cleanupCilium(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("cilium-cleanup")
	logger.Info("Starting Cilium cleanup")

	// Delete Cilium interfaces
	ciliumInterfaces := []string{"cilium_host", "cilium_net", "cilium_vxlan"}
	for _, iface := range ciliumInterfaces {
		if err := runCommand(logger, "ip", "link", "delete", iface); err != nil {
			logger.Info("Could not delete interface, it might not exist", "interface", iface)
		} else {
			logger.Info("Successfully deleted interface", "interface", iface)
		}
	}

	// Delete CNI config file
	cniConfigFile := "/etc/cni/net.d/05-cilium.conflist"
	if err := runCommand(logger, "rm", "-f", cniConfigFile); err != nil {
		logger.Error(err, "Failed to delete Cilium CNI config file", "file", cniConfigFile)
		return err
	}
	logger.Info("Successfully deleted Cilium CNI config file", "file", cniConfigFile)

	// TODO: Implement tc filter cleanup. This is critical.
	// Example: Find all interfaces with cilium's qdisc and detach it.
	// tc qdisc del dev <iface> clsact
	logger.Info("Skipping tc filter cleanup for now (TODO)")

	// TODO: Implement more robust iptables cleanup.
	// This requires finding all chains created by Cilium (e.g., CILIUM_FORWARD) and flushing/deleting them.
	// iptables-save | grep CILIUM | ...
	logger.Info("Skipping iptables cleanup for now (TODO)")

	// TODO: Implement eBPF cleanup.
	// This is the most complex part. It might involve unmounting the BPF filesystem,
	// or using 'bpftool' to detach programs and remove maps.
	// bpftool prog detach ...
	// bpftool map free ...
	logger.Info("Skipping eBPF cleanup for now (TODO)")

	logger.Info("Cilium cleanup finished")
	return nil
}

func cleanupSimpleBridge(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("simple-bridge-cleanup")
	logger.Info("Starting simple-bridge cleanup")

	// Delete bridge interface
	if err := runCommand(logger, "ip", "link", "delete", "cni0"); err != nil {
		logger.Info("Could not delete cni0 interface, it might not exist. This is probably okay.")
	} else {
		logger.Info("Successfully deleted cni0 interface")
	}

	// Delete CNI config file
	cniConfigFile := "/etc/cni/net.d/10-simple-bridge.conf"
	if err := runCommand(logger, "rm", "-f", cniConfigFile); err != nil {
		logger.Error(err, "Failed to delete simple-bridge CNI config file", "file", cniConfigFile)
		return err
	}
	logger.Info("Successfully deleted simple-bridge CNI config file", "file", cniConfigFile)

	// TODO: Implement iptables cleanup for simple-bridge.
	logger.Info("Skipping iptables cleanup for now (TODO)")

	logger.Info("simple-bridge cleanup finished")
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

	logger.Info("Command executed successfully", "stdout", stdout.String())
	return nil
}
