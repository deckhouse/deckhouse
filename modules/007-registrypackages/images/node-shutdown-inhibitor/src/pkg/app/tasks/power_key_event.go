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

package tasks

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"graceful_shutdown/pkg/inputdev"
)

// PowerKeyEvent is a task that listens for power key events.
// It lists all devices in /dev/input with power key.
// Then starts reading key event from all devices.
// If power key is pressed, it sends a shutdown event.
type PowerKeyEvent struct {
	// PowerKeyPressedCh is a channel to send power key press event.
	PowerKeyPressedCh  chan<- struct{}
	UnlockInhibitorsCh <-chan struct{}
	powerKeyDevices    []inputdev.Device
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
		// Trigger pod checker.
		fmt.Printf("powerKeyReader(s1): power key press detected, initiate graceful shutdown\n")
		err := exec.Command("systemctl", "poweroff", "--check-inhibitors=yes").Run()
		if err != nil {
			fmt.Printf("powerKeyReader(s1): poweroff error: %v\n", err)
		}
		//close(p.PowerKeyPressedCh)
	case <-p.UnlockInhibitorsCh:
		fmt.Printf("powerKeyReader(s1): shutdown initiated, stop power key reader loop\n")
		return
	}
	//
	//// Stage 2. Wait for pod checker and poweroff the system.
	//select {
	//case <-ctx.Done():
	//	fmt.Printf("powerKeyReader(s2): stop on global exit\n")
	//case <-p.UnlockInhibitorsCh:
	//	fmt.Printf("powerKeyReader(s2): pod lister meet shutdown requirements, poweroff the system now\n")
	//	fmt.Printf("/usr/sbin/systemctl poweroff --check-inhibitors=yes\n")
	//}
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

func (p *PowerKeyEvent) powerKeyPressed() chan struct{} {
	ch := make(chan struct{})

	go func() {
		// Open each device, select-read in loop to get input events, detect power key press.

		fmt.Printf("powerKeyReaderLoop: Wait for power key press\n")

		// time.Sleep(10 * time.Second)
		time.Sleep(24 * time.Hour)
		fmt.Printf("powerKeyReaderLoop: power key pressed\n")
		close(ch)
	}()

	return ch
}
