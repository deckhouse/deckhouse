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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

func runWrapper(done chan<- error, pidChan chan<- int) {
	cmd := exec.Command("/entrypoint", os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := cmd.Start()
	if err != nil {
		done <- err
		return
	}

	pidChan <- cmd.Process.Pid
	log.Printf("Wrapper started with PID: %d", cmd.Process.Pid)

	done <- cmd.Wait()
}

func deleteInterface() {
	interfaceName := "cni0"
	link, err := netlink.LinkByName(interfaceName)
	if err == nil {
		if err := netlink.LinkDel(link); err != nil {
			log.Printf("failed to delete interface %s: %v", link.Attrs().Name, err)
		} else {
			log.Printf("interface removed: %s", link.Attrs().Name)
		}
	}
}

func deleteConfigFiles() {
	configsDir := "/etc/cni/net.d/"
	files, _ := os.ReadDir(configsDir)
	for _, file := range files {
		if strings.Contains(file.Name(), "flannel") {
			fullPath := filepath.Join(configsDir, file.Name())
			if err := os.Remove(fullPath); err != nil {
				log.Printf("failed to delete configuration file %s: %v", fullPath, err)
			} else {
				log.Printf("configuration file removed: %s", fullPath)
			}
		}
	}
}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	pidChan := make(chan int, 1)
	wrapperDone := make(chan error, 1)
	go runWrapper(wrapperDone, pidChan)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v", sig)

		p, err := os.FindProcess(<-pidChan)
		if err == nil {
			if err := p.Signal(syscall.SIGTERM); err != nil {
				log.Printf("error sending SIGTERM to wrapper: %v", err)
			}
		}

		select {
		case err := <-wrapperDone:
			if err != nil {
				log.Printf("wrapper exited with error: %v", err)
			} else {
				log.Println("wrapper exited gracefully")
			}
		case <-time.After(15 * time.Second):
			log.Println("timeout waiting for wrapper to exit, killing it")
			if err := p.Kill(); err != nil {
				log.Printf("error killing wrapper: %v", err)
			}
		}

	case err := <-wrapperDone:
		if err != nil {
			log.Printf("wrapper exited with error: %v", err)
		} else {
			log.Println("wrapper exited")
		}
	}

	log.Println("start system cleaning")
	// deleteInterface()
	// deleteConfigFiles()
}
