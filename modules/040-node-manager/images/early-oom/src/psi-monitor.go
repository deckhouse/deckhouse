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
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

var fullAvg10Regex = regexp.MustCompile(`^full avg10=(\d{1,}\.\d{2}).*$`)

const (
	pollInterval     = 5 * time.Second
	recoveryInterval = 15 * time.Second
	memoryThreshold  = 5.00

	sysrqTriggerFile = "/proc/sysrq-trigger"
	psiMemoryFile    = "/proc/pressure/memory"
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
	psiContents, err := os.ReadFile(psiMemoryFile)
	if err != nil {
		log.Fatal(err)
	}

	avg10Values, err := parsePsiMemoryFile(psiContents)
	if err != nil {
		log.Fatal(err)
	}

	if avg10Values > memoryThreshold {
		log.Printf("full avg10 value %f, threshold %f, triggering system OOM killer...", avg10Values, memoryThreshold)
		err := triggerSystemOOM(sysrqTriggerFile)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Waiting for recovery for %s", recoveryInterval.String())

		return true
	}

	return false
}

func parsePsiMemoryFile(fileContents []byte) (float64, error) {
	var (
		fullAvg10Raw []byte
		fullAvg10    float64
	)

	scanner := bufio.NewScanner(bytes.NewReader(fileContents))
	for scanner.Scan() {
		matches := fullAvg10Regex.FindSubmatch(scanner.Bytes())
		if len(matches) == 2 {
			fullAvg10Raw = matches[1]
		}
	}
	err := scanner.Err()
	if err != nil {
		return 0, err
	}
	if len(fullAvg10Raw) == 0 {
		return 0, fmt.Errorf("can't parse %s", psiMemoryFile)
	}

	fullAvg10, err = strconv.ParseFloat(string(fullAvg10Raw), 64)
	if err != nil {
		return 0, err
	}

	return fullAvg10, nil
}

func triggerSystemOOM(sysrqTriggerFile string) error {
	return os.WriteFile(sysrqTriggerFile, []byte("f"), 0666)
}
