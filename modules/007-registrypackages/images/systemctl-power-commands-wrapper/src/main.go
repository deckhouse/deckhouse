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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const usage = `Wrapper for legacy power commands to invoke shutdown via logind
to send shutdown signal to all processes that hold inhibitor locks.
It translates legacy commands into:
systemctl halt|poweroff|reboot -i.

Create symlink with alias to invoke systemctl:

reboot                   Shut down and reboot the system
poweroff                 Shut down and power-off the system
shutdown                 Shut down and power-off the system
halt                     Shut down and halt the system

Options:
          --dry-run      Print systemctl command line, not run it.
   -r     --reboot       shutdown command compatibility: reboot.
   -P, -p --poweroff     halt command compatibility: poweroff.
   -H, -h --halt         poweroff command compatibility: halt.
   -f     --force        Force immediate halt/power-off/reboot.
   -n     --no-wall      Don't send wall message to users.
          --now          Execute the operation immediately (legacy compatibility).
          --no-block     Do not synchronously wait for operation to finish.
          --no-wtmp      Don't write wtmp record.
          --no-sync      Don't sync before halt/power-off/reboot.`

type Action int

const (
	ActionHalt Action = iota
	ActionPoweroff
	ActionReboot
)

func (a Action) String() string {
	switch a {
	case ActionHalt:
		return "halt"
	case ActionPoweroff:
		return "poweroff"
	case ActionReboot:
		return "reboot"
	default:
		return "unknown"
	}
}

type Config struct {
	action       Action
	dryRun       bool
	help         bool
	delaySeconds int
	unknownArgs  []string
}

func main() {
	config, err := parseArgs()
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	if config.help {
		fmt.Println(usage)
		os.Exit(0)
	}

	if config.delaySeconds > 0 {
		if err := scheduleShutdownViaLogind(config.action, config.delaySeconds); err != nil {
			fmt.Fprintf(os.Stderr, "schedule via logind: %v\n", err)
			os.Exit(1)
		}
		return
	}

	systemctlArgs := append([]string{"systemctl", config.action.String(), "-i"}, config.unknownArgs...)

	if config.dryRun {
		fmt.Printf("Cmd: %s\n", strings.Join(systemctlArgs, " "))
		return
	}

	cmd := exec.Command(systemctlArgs[0], systemctlArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "exec systemctl: %v\n", err)
		os.Exit(1)
	}
}

func scheduleShutdownViaLogind(action Action, delaySeconds int) error {
	target := action.String()
	when := time.Now().Add(time.Duration(delaySeconds) * time.Second)

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus: %w", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
	call := obj.Call("org.freedesktop.login1.Manager.ScheduleShutdown", 0, target, uint64(when.UnixMicro()))
	if call.Err != nil {
		return fmt.Errorf("ScheduleShutdown: %w", call.Err)
	}

	return nil
}

func parseArgs() (*Config, error) {
	config := &Config{}

	programName := filepath.Base(os.Args[0])

	switch programName {
	case "halt":
		config.action = ActionHalt
	case "poweroff":
		config.action = ActionPoweroff
	case "reboot":
		config.action = ActionReboot
	case "shutdown":
		config.action = ActionPoweroff
	default:
		config.help = true
		return config, nil
	}

	args := os.Args[1:]
	var knownArgs []string
	var unknownArgs []string

	// Define known flags that affect wrapper behavior
	knownFlags := map[string]bool{
		"--dry-run":  true,
		"--help":     true,
		"--reboot":   true,
		"-r":         true,
		"--poweroff": true,
		"-P":         true,
		"-p":         true,
		"--halt":     true,
		"-H":         true,
		"-h":         true,
		"-t":         true,
	}

	// Separate known and unknown arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Check if it's a known flag (handle --flag=value format)
		flagName := arg
		if strings.Contains(arg, "=") {
			flagName = strings.SplitN(arg, "=", 2)[0]
		}

		if knownFlags[flagName] {
			knownArgs = append(knownArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) {
				next := args[i+1]
				if !strings.HasPrefix(next, "-") {
					i++
					knownArgs = append(knownArgs, next)
				}
			}
		} else {
			unknownArgs = append(unknownArgs, arg)
		}
	}

	// Parse only known arguments
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.Usage = func() {
		config.help = true
	}

	var (
		dryRun         = fs.Bool("dry-run", false, "Print systemctl command line, not run it")
		help           = fs.Bool("help", false, "Show help")
		reboot         = fs.Bool("reboot", false, "shutdown command compatibility: reboot")
		rebootR        = fs.Bool("r", false, "shutdown command compatibility: reboot")
		poweroff       = fs.Bool("poweroff", false, "halt command compatibility: poweroff")
		poweroffP      = fs.Bool("P", false, "halt command compatibility: poweroff")
		poweroffLowerP = fs.Bool("p", false, "halt command compatibility: poweroff")
		halt           = fs.Bool("halt", false, "poweroff command compatibility: halt")
		haltH          = fs.Bool("H", false, "poweroff command compatibility: halt")
		haltLowerH     = fs.Bool("h", false, "poweroff command compatibility: halt")
		delay          = fs.Int("t", 0, "command delay compatibility in seconds")
	)

	// Suppress error output for unknown flags since we handle them separately
	fs.SetOutput(os.Stderr)

	if err := fs.Parse(knownArgs); err != nil {
		if err == flag.ErrHelp {
			config.help = true
			return config, nil
		}
		return nil, err
	}

	config.dryRun = *dryRun
	config.help = *help
	config.delaySeconds = *delay
	config.unknownArgs = unknownArgs

	// Handle action overrides from flags
	if *reboot || *rebootR {
		config.action = ActionReboot
	} else if *poweroff || *poweroffP || (*poweroffLowerP && config.action != ActionReboot) {
		config.action = ActionPoweroff
	} else if *halt || *haltH || (*haltLowerH && config.action != ActionHalt) {
		if config.action != ActionHalt {
			config.action = ActionPoweroff
		}
	}

	return config, nil
}
