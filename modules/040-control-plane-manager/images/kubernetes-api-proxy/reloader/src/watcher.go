/*
Copyright 2023 Flant JSC

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

package src

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
)

const (
	nginxConf    = "/etc/nginx/nginx.conf"
	nginxNewConf = "/etc/nginx/nginx_new.conf"
)

func nginxReload() error {
	// Check if nginx.conf has changed and test the new configuration
	changed, err := checkFileHashEquality(nginxConf, nginxNewConf)
	if err != nil {
		return err
	}
	if !changed {
		log.Printf("%s and %s are equal, skipping reload...", nginxConf, nginxNewConf)
	}

	output, err := exec.Command("nginx", "-t", "-c", nginxNewConf).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx configuration test failed: %s", string(output))
	}

	// Replace nginx.conf with nginx_new.conf and send SIGHUP signal to reload
	err = copyFile(nginxNewConf, nginxConf)
	if err != nil {
		return fmt.Errorf("failed to copy nginx_new.conf to nginx.conf: %s", err)
	}

	err = sendReloadSignal()
	if err != nil {
		return fmt.Errorf("failed to send SIGHUP to nginx process: %s", err)
	}

	return nil
}

// pkill -P vector SIGHUP
func sendReloadSignal() error {
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	for _, p := range processes {
		cmdline, err := p.Cmdline()
		if err != nil {
			return err
		}

		if strings.Contains(cmdline, "nginx") {
			err := p.SendSignal(unix.SIGHUP)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	source, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, source, 0644)
	if err != nil {
		return err
	}

	return nil
}

func WatchNginxConf() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(nginxNewConf)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// k8s configmaps use symlinks,
			// old file is deleted and a new link with the same name is created
			if event.Op == fsnotify.Remove {
				err := nginxReload()
				if err != nil {
					SetHealthCheckStatus(false)
					log.Printf("Failed to reload nginx: %s", err)
				} else {
					SetHealthCheckStatus(true)
				}

			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			SetHealthCheckStatus(false)
			log.Printf("Watcher error: %s", err)
		}
	}
}
