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
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
)

// The paths and the two side effects are variables rather than constants so
// that the reload logic can be driven in a test.
//
// Without a seam here the input file cannot be fuzzed at all -- the paths are
// absolute container paths, and sendReloadSignal walks the process table of
// whatever machine the test runs on and signals the first process whose command
// line mentions nginx. A test that reached it would signal an unrelated process
// on a developer's machine.
var (
	nginxConf    = "/etc/nginx/config/nginx.conf"
	nginxNewConf = "/etc/nginx/config/nginx_new.conf"

	// testConfig runs the configuration check that gates applying a new file.
	testConfig = func(path string) ([]byte, error) {
		// Force nginx to log config test errors to stderr.
		return exec.Command("nginx", "-t", "-c", path,
			"-e", "/dev/stderr", "-g", "error_log stderr;").CombinedOutput()
	}

	// signalReload tells the running nginx to re-read its configuration.
	signalReload = sendReloadSignal
)

func nginxReload() error {
	if _, err := os.Stat(nginxConf); errors.Is(err, fs.ErrNotExist) {
		err := copyFile(nginxNewConf, nginxConf)
		if err != nil {
			return err
		}
	}

	// Check if nginx.conf has changed and test the new configuration
	equal, err := fileContentsEqual(nginxConf, nginxNewConf)
	if equal {
		log.Printf("%s and %s are equal, skipping reload...", nginxConf, nginxNewConf)
		return nil
	}
	if err != nil {
		log.Printf("failed to check file equality: %s", err.Error())
	}

	log.Printf("%s differs from %s, validating and reloading nginx...", nginxNewConf, nginxConf)

	output, err := testConfig(nginxNewConf)
	if err != nil {
		return fmt.Errorf("nginx configuration test failed: %s", string(output))
	}

	// Replace nginx.conf with nginx_new.conf and send SIGHUP signal to reload
	err = copyFile(nginxNewConf, nginxConf)
	if err != nil {
		return fmt.Errorf("failed to copy nginx_new.conf to nginx.conf: %s", err)
	}

	err = signalReload()
	if err != nil {
		return fmt.Errorf("failed to send SIGHUP to nginx process: %s", err)
	}

	log.Printf("nginx reload finished successfully")
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

		if isNginxMasterCmdline(cmdline) {
			err := p.SendSignal(unix.SIGHUP)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// isNginxMasterCmdline reports whether a command line belongs to the nginx
// master process, which is the only process SIGHUP means "reload" to.
//
// The test is narrow on purpose. A substring match on "nginx" also matches a
// worker, for which SIGHUP means "shut down gracefully"; the `nginx -t` child
// this reloader spawns itself; and any unrelated process that merely mentions
// the word, such as a tail of an nginx log. sendReloadSignal stops at the first
// match, so one of those matching first means the master is never signalled and
// the new configuration is never applied -- with the copy already done, so the
// files agree and the next event skips the reload as well.
//
// nginx rewrites its process title, so the master reads as
// "nginx: master process <path> ..." and a worker as "nginx: worker process".
// The bare executable form is accepted too, for the window before the title is
// rewritten: the container entrypoint is
// ["/opt/nginx-static/sbin/nginx", "-g", "daemon off;"].
func isNginxMasterCmdline(cmdline string) bool {
	if strings.HasPrefix(cmdline, "nginx: master process") {
		return true
	}

	// Before the title is rewritten the command line is the executable and its
	// arguments. Accept that only when the executable itself is nginx, not when
	// nginx appears somewhere in the arguments.
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return false
	}
	if filepath.Base(fields[0]) != "nginx" {
		return false
	}
	// `nginx -t` and `nginx -s ...` are one-shot invocations, not the master.
	for _, arg := range fields[1:] {
		switch arg {
		case "-t", "-T", "-s", "-v", "-V", "-h", "-?":
			return false
		}
	}
	return true
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
	err := nginxReload()
	if err != nil {
		log.Fatal(err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.Add(filepath.Dir(nginxNewConf))
	if err != nil {
		watcher.Close()
		log.Fatal(err)
	}
	defer watcher.Close()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Name == nginxNewConf {
				err := nginxReload()
				if err != nil {
					log.Printf("Failed to reload nginx: %s", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			log.Printf("Watcher error: %s", err)
		}
	}
}
