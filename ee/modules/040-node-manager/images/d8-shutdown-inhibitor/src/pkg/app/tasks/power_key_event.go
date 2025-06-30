/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"
	"os/exec"

	"d8_shutdown_inhibitor/pkg/inputdev"
)

// PowerKeyEvent is a task that listens for power key events.
// It lists all devices in /dev/input with power key.
// Then starts reading key event from all devices.
// If power key is pressed, it sends a shutdown event.
type PowerKeyEvent struct {
	// PowerKeyPressedCh is a channel to send power key press event.
	PowerKeyPressedCh  chan<- struct{}
	UnlockInhibitorsCh <-chan struct{}

	powerKeyDevices []inputdev.Device
}

func (p *PowerKeyEvent) Name() string {
	return "powerKeyReader"
}

func (p *PowerKeyEvent) Run(ctx context.Context, errCh chan error) {
	// List all devices in /dev/input
	err := p.prepare()
	if err != nil {
		errCh <- fmt.Errorf("powerKeyReader prepare: %w", err)
		return
	}

	powerKeyWatcher := inputdev.NewWatcher(p.powerKeyDevices, inputdev.KEY_POWER, inputdev.KEY_POWER2)
	powerKeyWatcher.Start()
	defer powerKeyWatcher.Stop()

	select {
	case <-ctx.Done():
		fmt.Printf("powerKeyReader(s1): stop on global exit\n")
		return
	case <-powerKeyWatcher.Pressed():
		// Trigger poweroff to ShutdownInhibitor catch the PrepareShutdownSignal from logind.
		fmt.Printf("powerKeyReader(s1): power key press detected, initiate graceful shutdown\n")
		// Run systemctl poweroff -i so systemd will send shutdown signal to all inhibit locks holders
		// (ShutdownInhibitor task will catch it as well as a kubelet).
		err := exec.Command("systemctl", "poweroff", "-i").Run()
		if err != nil {
			fmt.Printf("powerKeyReader(s1): poweroff error: %v\n", err)
		}
	case <-p.UnlockInhibitorsCh:
		fmt.Printf("powerKeyReader(s1): shutdown initiated, stop power key reader loop\n")
		return
	}
}

// prepare lists input devices to detect devices with power key.
func (p *PowerKeyEvent) prepare() error {
	powerKeyDevices, err := inputdev.ListInputDevicesWithAnyButton(inputdev.KEY_POWER, inputdev.KEY_POWER2)
	if err != nil {
		return fmt.Errorf("list power key devices: %w", err)
	}
	p.powerKeyDevices = powerKeyDevices
	return nil
}
