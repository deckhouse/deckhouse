/*
Copyright 2021 Flant JSC

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
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/procfs"
)

const (
	pollInterval     = 5 * time.Second
	recoveryInterval = 15 * time.Second
	memoryThreshold  = 5.00

	sysrqTriggerFile  = "/proc/sysrq-trigger"
	sysrqOOMCharacter = "f"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	t := time.NewTimer(0)
	for {
		select {
		case sig := <-c:
			log.Printf("Caught signal %s, exiting", sig.String())
			os.Exit(0)
		case <-t.C:
			if iteration() {
				t.Reset(recoveryInterval)
			} else {
				t.Reset(pollInterval)
			}
		}
	}
}

func iteration() bool {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		log.Fatal(err)
	}

	stats, err := fs.PSIStatsForResource("memory")
	if err != nil {
		log.Fatal(err)
	}

	if stats.Full.Avg10 > memoryThreshold {
		log.Printf("full avg10 value %f, threshold %f, triggering system OOM killer...", stats.Full.Avg10, memoryThreshold)
		err := triggerSystemOOM(sysrqTriggerFile)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Waiting for recovery for %s", recoveryInterval.String())

		return true
	}

	return false
}

func triggerSystemOOM(sysrqTriggerFile string) error {
	return os.WriteFile(sysrqTriggerFile, []byte(sysrqOOMCharacter), 0666)
}
